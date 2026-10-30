"""Bounded native-dyld symbol lookup; no execution or host memory access."""
import struct

MAX_DYLD_BYTES = 64 << 20
EXPECTED_SYMBOL = b"___chkstk_darwin\0"


def stack_probe_offset(data, expected_uuid):
    if len(data) > MAX_DYLD_BYTES:
        raise ValueError("dyld exceeds symbol-capture bound")

    def span(blob, offset, size):
        if offset < 0 or size < 0 or offset > len(blob) or size > len(blob)-offset:
            raise ValueError("dyld metadata outside file")
        return blob[offset:offset+size]

    slices = [data]
    if data[:4] in (b"\xca\xfe\xba\xbe", b"\xca\xfe\xba\xbf"):
        count = struct.unpack(">I", span(data, 4, 4))[0]
        if not 1 <= count <= 32:
            raise ValueError("invalid dyld universal count")
        wide = data[:4] == b"\xca\xfe\xba\xbf"
        entry_size = 32 if wide else 20
        table = span(data, 8, count*entry_size)
        slices = []
        ranges = []
        for i in range(count):
            entry = table[i*entry_size:(i+1)*entry_size]
            cpu, subtype = struct.unpack_from(">II", entry)
            offset, size = struct.unpack_from(">QQ" if wide else ">II", entry, 8)
            alignment=struct.unpack_from(">I",entry,24 if wide else 16)[0]
            if alignment>31 or offset%(1<<alignment) or offset<8+len(table) or size<32:
                raise ValueError("invalid dyld universal slice extent/alignment")
            if wide and struct.unpack_from(">I",entry,28)[0]:
                raise ValueError("invalid dyld universal reserved field")
            if any(offset<end and start<offset+size for start,end in ranges):
                raise ValueError("overlapping dyld universal slices")
            ranges.append((offset,offset+size))
            blob = span(data, offset, size)
            if cpu != 0x100000c:
                continue
            if len(blob) < 32 or struct.unpack_from("<II", blob, 4) != (cpu, subtype):
                raise ValueError("dyld universal/thin CPU mismatch")
            slices.append(blob)
    matches = []
    for blob in slices:
        header = struct.unpack("<8I", span(blob, 0, 32))
        if header[0] != 0xfeedfacf or header[1] != 0x100000c or header[3] != 7:
            raise ValueError("invalid native ARM64 dyld header")
        count, size = header[4:6]
        if not 1 <= count <= 4096 or size > 1 << 20:
            raise ValueError("invalid dyld load commands")
        commands = span(blob, 32, size)
        cursor = 0
        uuids, tables, text_bases, sections = [], [], [], []
        for _ in range(count):
            command, length = struct.unpack("<II", span(commands, cursor, 8))
            if length < 8 or length % 8:
                raise ValueError("invalid dyld command size")
            value = span(commands, cursor, length)
            if command == 0x1b:
                if length != 24:
                    raise ValueError("invalid dyld UUID command")
                uuids.append(value[8:].hex())
            elif command == 2:
                if length != 24:
                    raise ValueError("invalid dyld symbol command")
                tables.append(struct.unpack_from("<4I", value, 8))
            elif command == 0x19:
                if length < 72:
                    raise ValueError("invalid dyld segment")
                vmaddr, vmsize, fileoff, filesize = struct.unpack_from("<4Q", value, 24)
                maxprot, prot, nsects = struct.unpack_from("<3I", value, 56)
                if nsects > 255 or length != 72+nsects*80 or prot & ~maxprot or vmaddr+vmsize > 1<<64:
                    raise ValueError("invalid dyld section table")
                span(blob, fileoff, filesize)
                if value[8:24].rstrip(b"\0") == b"__TEXT":
                    if fileoff != 0:
                        raise ValueError("dyld text does not contain image header")
                    text_bases.append(vmaddr)
                for j in range(nsects):
                    section = value[72+j*80:72+(j+1)*80]
                    address, extent, offset = struct.unpack_from("<QQI", section, 32)
                    if address < vmaddr or address-vmaddr > vmsize or extent > vmsize-(address-vmaddr):
                        raise ValueError("dyld section outside segment")
                    sections.append((address, extent, prot, offset, fileoff, filesize))
            cursor += length
        if cursor != len(commands) or len(uuids) != 1:
            raise ValueError("ambiguous dyld UUID/command extent")
        if uuids[0] != expected_uuid:
            continue
        if len(tables) != 1 or len(text_bases) != 1:
            raise ValueError("ambiguous dyld symbol/text metadata")
        symoff, nsyms, stroff, strsize = tables[0]
        if nsyms > 1 << 20 or strsize > 32 << 20:
            raise ValueError("dyld symbol table exceeds bound")
        symbols, strings = span(blob, symoff, nsyms*16), span(blob, stroff, strsize)
        found = []
        for i in range(nsyms):
            index, kind, section, _, value = struct.unpack_from("<IBBHQ", symbols, i*16)
            if index >= len(strings):
                raise ValueError("dyld symbol string outside table")
            if strings[index:index+len(EXPECTED_SYMBOL)] != EXPECTED_SYMBOL:
                continue
            if kind & 0xe0 or kind & 0xe != 0xe or not 1 <= section <= len(sections):
                raise ValueError("stack probe is not a defined section symbol")
            address, extent, prot, offset, fileoff, filesize = sections[section-1]
            if not prot & 4 or value < address or value-address > extent or extent-(value-address) < 8:
                raise ValueError("stack probe outside executable section")
            location = offset+value-address
            if location < fileoff or location-fileoff > filesize or filesize-(location-fileoff) < 8:
                raise ValueError("stack probe outside file-backed segment")
            if span(blob, location, 4) != bytes.fromhex("5f2403d5") or value < text_bases[0]:
                raise ValueError("stack probe lacks expected BTI c entry")
            found.append(value-text_bases[0])
        if len(found) != 1:
            raise ValueError("missing/ambiguous stack probe symbol")
        matches.append(found[0])
    if len(matches) != 1:
        raise ValueError("missing/ambiguous matching native dyld UUID")
    return matches[0]

ENTRY(_start)
PHDRS {
  code PT_LOAD FILEHDR PHDRS FLAGS(5);
  constant PT_LOAD FLAGS(4);
  data PT_LOAD FLAGS(6);
  stack PT_GNU_STACK FLAGS(6);
}
SECTIONS {
  . = 0x400000 + SIZEOF_HEADERS;
  .text : { *(.text*) } :code
  . = ALIGN(4096);
  .rodata : { *(.rodata*) } :constant
  . = ALIGN(4096);
  .data : { *(.data*) } :data
  .bss (NOLOAD) : { *(.bss*) } :data
  /DISCARD/ : { *(.comment) *(.note*) }
}

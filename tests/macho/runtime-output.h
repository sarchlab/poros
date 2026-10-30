// macOS platform output: keep the actual libSystem call in the guest program.
extern long write(int fd, const void *data, unsigned long bytes);
static int output(const void *data, u64 bytes) {
    return write(1, data, bytes) == (long)bytes ? 0 : 3;
}

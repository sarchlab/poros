// Ordinary C main reached by the real macOS dyld/libSystem startup.
// Only adapts main's argument convention to the shared benchmark function.
extern int guest_main(unsigned long *);
int main(int argc, char **argv) {
    if (argc < 1 || argc > 3) return 2;
    unsigned long arguments[5] = {(unsigned long)argc, 0, 0, 0, 0};
    for (int i = 0; i < argc; i++) arguments[i + 1] = (unsigned long)argv[i];
    return guest_main(arguments);
}

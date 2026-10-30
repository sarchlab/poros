// Selected capability byte identified at the native libdyld
// _dyld_call_with_writable_tpro_memory entry; no register writes or JIT mapping.
#include <stdint.h>
#include <stdio.h>
int main(void){printf("{\"offset\":268,\"width\":1,\"value\":%u}\n",*(const volatile uint8_t*)0xfffffc10cULL);return 0;}

// Selected documented byte only: XNU osfmk/arm/cpu_capabilities.h
// _COMM_PAGE_DTRACE_DOF_ENABLED = _COMM_PAGE_START_ADDRESS + 0x092.
#include <stdint.h>
#include <stdio.h>
int main(void){const volatile uint8_t*p=(const volatile uint8_t*)0xfffffc000ULL;printf("{\"offset\":146,\"width\":1,\"dtrace_dof_enabled\":%u}\n",p[0x92]);return 0;}

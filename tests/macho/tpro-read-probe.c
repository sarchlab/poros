#include <stdint.h>
#include <stdio.h>
int main(void){uint8_t capability=*(const volatile uint8_t*)0xfffffc10cULL; if(capability!=3)return 2; uint64_t value; __asm__ volatile("mrs %0, S3_6_C15_C1_5":"=r"(value)); printf("{\"capability\":%u,\"register\":\"S3_6_C15_C1_5\",\"value\":\"0x%016llx\",\"bit36\":%u}\n",capability,(unsigned long long)value,(unsigned)((value>>36)&1)); return 0;}

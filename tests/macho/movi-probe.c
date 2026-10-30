#include <stdint.h>
#include <stdio.h>
#define PROBE(LANES,SHIFT,IMM) do {uint8_t b[16];__asm__ volatile("movi v0.16b,#255\n\tmovi v0." #LANES ",#" #IMM ",lsl #" #SHIFT "\n\tstr q0,[%0]" :: "r"(b):"v0","memory");printf("%s{\"lanes\":%u,\"shift\":%u,\"immediate\":%u,\"bytes\":\"",index++?",":"",count,SHIFT,IMM);for(unsigned j=0;j<16;j++)printf("%02x",b[j]);printf("\"}");}while(0)
#define SET(LANES,IMM) PROBE(LANES,0,IMM);PROBE(LANES,8,IMM);PROBE(LANES,16,IMM);PROBE(LANES,24,IMM)
int main(void){unsigned index=0,count=2;puts("{\"vectors\":[");SET(2s,1);SET(2s,128);SET(2s,165);SET(2s,255);count=4;SET(4s,1);SET(4s,128);SET(4s,165);SET(4s,255);puts("]}");return 0;}

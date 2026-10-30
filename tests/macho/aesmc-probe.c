#include <stdint.h>
#include <stdio.h>
#include <string.h>
typedef void(*runner)(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_0(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_1(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_2(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_3(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
static struct {const char*kind;unsigned word;runner run;}code[]={
{"AESMC",0x4e286800,run_0},
{"AESMC",0x4e286841,run_1},
{"AESMC",0x4e28681f,run_2},
{"AESMC",0x4e286be0,run_3},
};static void hex(const unsigned char*b,unsigned n){for(unsigned i=0;i<n;i++)printf("%02x",b[i]);}
static void fill(unsigned char*b,unsigned p){for(unsigned r=0;r<32;r++)for(unsigned i=0;i<16;i++)b[16*r+i]=(p>=32?(i==(p-32)/8?1U<<((p-32)%8):0):(p<16?16*p+i:(p*73+r*29+i*17+(i*r))))&255;}
int main(void){_Alignas(16) unsigned char initial[512],out[512];unsigned count=0;printf("{\"patterns\":[");for(unsigned p=0;p<160;p++){fill(initial,p);printf("%s\"",p?",":"");hex(initial,512);printf("\"");}printf("],\"cases\":[");for(unsigned c=0;c<sizeof(code)/sizeof(code[0]);c++)for(unsigned p=0;p<160;p++){fill(initial,p);unsigned rd=code[c].word&31;uint64_t before=(uint64_t)(p%16)<<28,after=0;code[c].run(initial,out,before,&after);for(unsigned r=0;r<32;r++)if(r!=rd&&memcmp(initial+16*r,out+16*r,16))return 3;if(before!=after)return 4;printf("%s{\"kind\":\"%s\",\"encoding\":%u,\"pattern\":%u,\"input_nzcv\":%llu,\"output_nzcv\":%llu,\"untouched_match\":true,\"output_hex\":\"",count++?",":"",code[c].kind,code[c].word,p,before,after);hex(out+16*rd,16);printf("\"}");}printf("],\"case_count\":%u}\n",count);}

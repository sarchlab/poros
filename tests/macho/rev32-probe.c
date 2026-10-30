#include <stdint.h>
#include <stdio.h>
#include <string.h>
typedef void(*runner)(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_0(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_1(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_2(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_3(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_4(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_5(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_6(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_7(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_8(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_9(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_10(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_11(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_12(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_13(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_14(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_15(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
static struct {const char*kind;unsigned word;runner run;}code[]={
{"REV32",0x2e200800,run_0},
{"REV32",0x2e200841,run_1},
{"REV32",0x2e20081f,run_2},
{"REV32",0x2e200be0,run_3},
{"REV32",0x2e600800,run_4},
{"REV32",0x2e600841,run_5},
{"REV32",0x2e60081f,run_6},
{"REV32",0x2e600be0,run_7},
{"REV32",0x6e200800,run_8},
{"REV32",0x6e200841,run_9},
{"REV32",0x6e20081f,run_10},
{"REV32",0x6e200be0,run_11},
{"REV32",0x6e600800,run_12},
{"REV32",0x6e600841,run_13},
{"REV32",0x6e60081f,run_14},
{"REV32",0x6e600be0,run_15},
};static void hex(const unsigned char*b,unsigned n){for(unsigned i=0;i<n;i++)printf("%02x",b[i]);}
static void fill(unsigned char*b,unsigned p){for(unsigned r=0;r<32;r++)for(unsigned i=0;i<16;i++)b[16*r+i]=(p>=32?(p==32?0:p==33?255:p==34?128:1):(p<16?16*p+i:(p*73+r*29+i*17+(i*r))))&255;}
int main(void){_Alignas(16) unsigned char initial[512],out[512];unsigned count=0;printf("{\"patterns\":[");for(unsigned p=0;p<36;p++){fill(initial,p);printf("%s\"",p?",":"");hex(initial,512);printf("\"");}printf("],\"cases\":[");for(unsigned c=0;c<sizeof(code)/sizeof(code[0]);c++)for(unsigned p=0;p<36;p++){fill(initial,p);unsigned rd=code[c].word&31;uint64_t before=(uint64_t)(p%16)<<28,after=0;code[c].run(initial,out,before,&after);for(unsigned r=0;r<32;r++)if(r!=rd&&memcmp(initial+16*r,out+16*r,16))return 3;if(before!=after)return 4;printf("%s{\"kind\":\"%s\",\"encoding\":%u,\"pattern\":%u,\"input_nzcv\":%llu,\"output_nzcv\":%llu,\"untouched_match\":true,\"output_hex\":\"",count++?",":"",code[c].kind,code[c].word,p,before,after);hex(out+16*rd,16);printf("\"}");}printf("],\"case_count\":%u}\n",count);}

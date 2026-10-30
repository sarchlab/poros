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
extern void run_16(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_17(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_18(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_19(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_20(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_21(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_22(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_23(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_24(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_25(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_26(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_27(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_28(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_29(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_30(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_31(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_32(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_33(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
extern void run_34(const unsigned char*,unsigned char*,uint64_t,uint64_t*);
static struct {const char*kind;unsigned word;runner run;}code[]={
{"ADDvector",0x0e228420,run_0},
{"ADDvector",0x0e228421,run_1},
{"ADDvector",0x0e228422,run_2},
{"ADDvector",0x0e3f87ff,run_3},
{"ADDvector",0x0e3e861e,run_4},
{"ADDvector",0x0e628420,run_5},
{"ADDvector",0x0e628421,run_6},
{"ADDvector",0x0e628422,run_7},
{"ADDvector",0x0e7f87ff,run_8},
{"ADDvector",0x0e7e861e,run_9},
{"ADDvector",0x0ea28420,run_10},
{"ADDvector",0x0ea28421,run_11},
{"ADDvector",0x0ea28422,run_12},
{"ADDvector",0x0ebf87ff,run_13},
{"ADDvector",0x0ebe861e,run_14},
{"ADDvector",0x4e228420,run_15},
{"ADDvector",0x4e228421,run_16},
{"ADDvector",0x4e228422,run_17},
{"ADDvector",0x4e3f87ff,run_18},
{"ADDvector",0x4e3e861e,run_19},
{"ADDvector",0x4e628420,run_20},
{"ADDvector",0x4e628421,run_21},
{"ADDvector",0x4e628422,run_22},
{"ADDvector",0x4e7f87ff,run_23},
{"ADDvector",0x4e7e861e,run_24},
{"ADDvector",0x4ea28420,run_25},
{"ADDvector",0x4ea28421,run_26},
{"ADDvector",0x4ea28422,run_27},
{"ADDvector",0x4ebf87ff,run_28},
{"ADDvector",0x4ebe861e,run_29},
{"ADDvector",0x4ee28420,run_30},
{"ADDvector",0x4ee28421,run_31},
{"ADDvector",0x4ee28422,run_32},
{"ADDvector",0x4eff87ff,run_33},
{"ADDvector",0x4efe861e,run_34},
};static void hex(const unsigned char*b,unsigned n){for(unsigned i=0;i<n;i++)printf("%02x",b[i]);}
static void fill(unsigned char*b,unsigned p){for(unsigned r=0;r<32;r++)for(unsigned i=0;i<16;i++)b[16*r+i]=(p>=32?(p==32?0:p==33?255:p==34?128:1):(p<16?16*p+i:(p*73+r*29+i*17+(i*r))))&255;}
int main(void){_Alignas(16) unsigned char initial[512],out[512];unsigned count=0;printf("{\"patterns\":[");for(unsigned p=0;p<36;p++){fill(initial,p);printf("%s\"",p?",":"");hex(initial,512);printf("\"");}printf("],\"cases\":[");for(unsigned c=0;c<sizeof(code)/sizeof(code[0]);c++)for(unsigned p=0;p<36;p++){fill(initial,p);unsigned rd=code[c].word&31;uint64_t before=(uint64_t)(p%16)<<28,after=0;code[c].run(initial,out,before,&after);for(unsigned r=0;r<32;r++)if(r!=rd&&memcmp(initial+16*r,out+16*r,16))return 3;if(before!=after)return 4;printf("%s{\"kind\":\"%s\",\"encoding\":%u,\"pattern\":%u,\"input_nzcv\":%llu,\"output_nzcv\":%llu,\"untouched_match\":true,\"output_hex\":\"",count++?",":"",code[c].kind,code[c].word,p,before,after);hex(out+16*rd,16);printf("\"}");}printf("],\"case_count\":%u}\n",count);}

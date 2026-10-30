#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <limits.h>
#include <string.h>
#include <dlfcn.h>
#include <mach-o/loader.h>
#include <mach/mach.h>
#include <mach/mach_vm.h>
extern const struct mach_header_64 _mh_execute_header;
static const char marker[]="poros chain page native oracle";
__attribute__((used,section("__DATA_CONST,__probe"))) static const void *const pointers[]={marker,marker+7,(void*)puts,(char*)(void*)puts+7,(char*)(void*)puts+255,(void*)printf,(void*)malloc};
static void *expected(unsigned i){switch(i){case 0:return(void*)marker;case 1:return(void*)(marker+7);case 2:return dlsym(RTLD_DEFAULT,"puts");case 3:return(char*)dlsym(RTLD_DEFAULT,"puts")+7;case 4:return(char*)dlsym(RTLD_DEFAULT,"puts")+255;case 5:return dlsym(RTLD_DEFAULT,"printf");default:return dlsym(RTLD_DEFAULT,"malloc");}}
int main(int argc,char**argv){if(argc!=1)return 2;char path[PATH_MAX];if(!realpath(argv[0],path))return 3;int fd=open(path,O_RDONLY);if(fd<0)return 4;const unsigned char*commands=(void*)(&_mh_execute_header+1);uint64_t preferred=0,fileoff=0,sectionaddr=0;unsigned format=0;
for(unsigned i=0;i<_mh_execute_header.ncmds;i++){const struct load_command*l=(void*)commands;if(l->cmd==LC_SEGMENT_64){const struct segment_command_64*s=(void*)l;if(!strcmp(s->segname,"__TEXT"))preferred=s->vmaddr;const struct section_64*sects=(void*)(s+1);for(unsigned j=0;j<s->nsects;j++)if(!strcmp(sects[j].sectname,"__probe")){fileoff=sects[j].offset;sectionaddr=sects[j].addr;}}if(l->cmd==LC_DYLD_CHAINED_FIXUPS){const struct linkedit_data_command*c=(void*)l;unsigned char h[64];if(pread(fd,h,64,c->dataoff)!=64)return 5;uint32_t starts;memcpy(&starts,h+4,4);if(starts>c->datasize-8)return 5;uint32_t n;if(pread(fd,&n,4,c->dataoff+starts)!=4||n>64||4+4*n>c->datasize-starts)return 5;for(uint32_t j=0;j<n;j++){uint32_t at;if(pread(fd,&at,4,c->dataoff+starts+4+4*j)!=4)return 5;if(!at)continue;if(at>c->datasize-starts-8)return 5;uint16_t f;if(pread(fd,&f,2,c->dataoff+starts+at+6)!=2)return 5;if(format&&format!=f)return 5;format=f;}}commands+=l->cmdsize;}
if(!preferred||!fileoff||!sectionaddr||format!=6)return 6;uint64_t slide=(uintptr_t)&_mh_execute_header-preferred;if((uintptr_t)pointers!=sectionaddr+slide)return 7;
mach_vm_address_t address=(uintptr_t)pointers;mach_vm_size_t size;vm_region_basic_info_data_64_t info;mach_msg_type_number_t count=VM_REGION_BASIC_INFO_COUNT_64;mach_port_t object=0;if(mach_vm_region(mach_task_self(),&address,&size,VM_REGION_BASIC_INFO_64,(vm_region_info_t)&info,&count,&object))return 8;if(object)mach_port_deallocate(mach_task_self(),object);
printf("{\"preferred_base\":\"%016llx\",\"runtime_base\":\"%016llx\",\"slide\":\"%016llx\",\"pointer_format\":%u,\"section_file_offset\":%llu,\"protection\":%d,\"max_protection\":%d,\"cases\":[",preferred,(uint64_t)(uintptr_t)&_mh_execute_header,slide,format,fileoff,info.protection,info.max_protection);
for(unsigned i=0;i<sizeof(pointers)/sizeof(pointers[0]);i++){uint64_t encoded,actual,want=(uintptr_t)expected(i);if(pread(fd,&encoded,8,fileoff+8*i)!=8)return 9;actual=*(volatile const uint64_t*)((uintptr_t)pointers+i*8);if(actual!=want)return 10;printf("%s{\"index\":%u,\"encoding\":\"%016llx\",\"actual\":\"%016llx\",\"expected\":\"%016llx\"}",i?",":"",i,encoded,actual,want);}puts("]}");return close(fd);}

// Capture only documented nonsecret launch metadata from a newly spawned,
// suspended child. Fixed argv and an empty environment prevent application
// arguments from being mistaken for kernel apple keys. The child never runs.
#include <spawn.h>
#include <sys/sysctl.h>
#include <sys/wait.h>
#include <signal.h>
#include <unistd.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <ctype.h>

static const char *keys[] = {"pfz", "stack_guard", "malloc_entropy", "ptr_munge",
    "main_stack", "executable_file", "dyld_file", "executable_cdhash",
    "executable_boothash", "th_port", "security_config", "arm64e_abi",
    "ptrauth_disabled", "executable_path"};
// Query only the owned suspended child's permission, before any dyld code runs.
// This records that executable's policy rather than assuming the helper's.
static int syscall_permission(pid_t child, uint64_t *decision) {
    uint64_t output[4]={UINT64_MAX,UINT64_MAX,UINT64_MAX,UINT64_MAX};
    uint64_t request[20]={0};
    request[0]=(uintptr_t)output;request[1]=(uint64_t)child;
    request[2]=(uintptr_t)"syscall-unix";request[3]=0x41;
    request[4]=550;request[5]=1;
    register uint64_t a __asm__("x0")=(uintptr_t)"Sandbox";
    register uint64_t b __asm__("x1")=2,c __asm__("x2")=(uintptr_t)request;
    register uint64_t number __asm__("x16")=381;uint64_t flags;
    __asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(flags):"r"(c),"r"(number):"memory","cc");
    if(a||b||(flags&(1ull<<29))||output[1]!=UINT64_MAX||output[2]!=UINT64_MAX||output[3]!=UINT64_MAX)return -1;
    *decision=output[0];return 0;
}
static int reap(pid_t child) {
    if (kill(child, SIGKILL) != 0 && errno != ESRCH) return -1;
    int status; pid_t got;
    do { got = waitpid(child, &status, 0); } while (got < 0 && errno == EINTR);
    return got == child ? 0 : -1;
}
static int skip_string(char *b, size_t n, size_t *p, char **value) {
    if (*p >= n) return -1;
    size_t length = strnlen(b + *p, n - *p);
    if (length == n - *p) return -1;
    *value = b + *p; *p += length + 1; return 0;
}
static int hex_value(const char *s, uint64_t *value) {
    if (strncmp(s,"0x",2) != 0 || !s[2]) return -1;
    errno=0; char *end; unsigned long long v=strtoull(s,&end,16);
    if (errno || *end) return -1;
    *value=(uint64_t)v; return 0;
}
static int emit(char *buffer, size_t size, uint64_t decision) {
    if (size < 4) return -1;
    int argc; memcpy(&argc,buffer,4); if (argc != 2) return -1;
    size_t pos=4; char *s;
    if (skip_string(buffer,size,&pos,&s) || s[0]!='/') return -1;
    while (pos<size && buffer[pos]==0) pos++;
    const char *args[]={"program","1"};
    for (unsigned i=0;i<2;i++) if (skip_string(buffer,size,&pos,&s)||strcmp(s,args[i])) return -1;
    char *values[sizeof(keys)/sizeof(keys[0])] = {0};
    while (pos<size) {
        if (skip_string(buffer,size,&pos,&s)) return -1;
        if (!*s) continue;
        char *equal=strchr(s,'='); if (!equal) return -1;
        size_t length=(size_t)(equal-s); unsigned k;
        for(k=0;k<sizeof(keys)/sizeof(keys[0]);k++) if(strlen(keys[k])==length&&!memcmp(s,keys[k],length))break;
        if(k==sizeof(keys)/sizeof(keys[0]) || values[k]) return -1;
        values[k]=equal+1;
    }
    for(unsigned k=0;k<11;k++) if(!values[k])return -1;
    uint64_t numbers[14][4]={{0}};
    for(unsigned k=0;k<14;k++) {
        if(!values[k] || k==1 || k==2 || k==3 || k==13) continue;
        if(k==7 || k==8) {
            if(strlen(values[k])!=40)return -1;
            for(unsigned i=0;i<40;i++)if(!isxdigit((unsigned char)values[k][i]))return -1;
        } else if(k==11) {if(strcmp(values[k],"all")&&strcmp(values[k],"os"))return -1;
        } else if(k==12) {if(strcmp(values[k],"1"))return -1;
        } else {
            unsigned count=k==4?4:(k==5||k==6)?2:1;
            char *p=values[k];
            for(unsigned j=0;j<count;j++) {
                char *comma=strchr(p,','); if(comma)*comma=0;
                if(hex_value(p,&numbers[k][j]))return -1;
                if((j+1<count)!=(comma!=NULL))return -1;
                if(comma){*comma=',';p=comma+1;}
            }
        }
    }
    if(numbers[4][0]<numbers[4][2])return -1;
    printf("{\"filtered_apple\":["); unsigned emitted=0;
    for(unsigned k=0;k<14;k++) {
        if(!values[k])continue;
        printf("%s{\"key\":\"%s\"",emitted++?",":"",keys[k]);
        if(k==1||k==2||k==3||k==13)printf(",\"value_omitted\":true");
        else if(k==4)printf(",\"stack_size\":%llu,\"allocation_size\":%llu,\"top_minus_allocation_base\":%llu",(unsigned long long)numbers[k][1],(unsigned long long)numbers[k][3],(unsigned long long)(numbers[k][0]-numbers[k][2]));
        else printf(",\"value\":\"%s\"",values[k]);
        printf("}");
    }
    printf("],\"sandbox_syscall_permissions\":[{\"number\":550,\"decision\":\"%llu\"}]}\n",(unsigned long long)decision); return 0;
}
int main(int argc,char **argv) {
    if(argc!=2 || argv[1][0]!='/')return 2;
    size_t capacity=1<<20;char *buffer=calloc(1,capacity);if(!buffer)return 3;
    posix_spawnattr_t attr;int error=posix_spawnattr_init(&attr);
    if(error){free(buffer);return 4;}
    error=posix_spawnattr_setflags(&attr,POSIX_SPAWN_START_SUSPENDED);
    pid_t child=-1;char *args[]={"program","1",NULL};char *env[]={NULL};
    if(!error)error=posix_spawn(&child,argv[1],NULL,&attr,args,env);
    posix_spawnattr_destroy(&attr);
    if(error){free(buffer);fprintf(stderr,"startup capture spawn failed: %d\n",error);return 5;}
    int mib[]={CTL_KERN,KERN_PROCARGS2,child};size_t size=capacity;
    int result=sysctl(mib,3,buffer,&size,NULL,0);int saved=errno;
    uint64_t decision=0;int policy=result?0:syscall_permission(child,&decision);
    if(reap(child)){free(buffer);fprintf(stderr,"startup capture could not reap its child\n");return 6;}
    if(result||size>capacity){free(buffer);fprintf(stderr,"startup capture query failed: %d\n",saved);return 7;}
    if(policy){memset(buffer,0,capacity);free(buffer);fprintf(stderr,"owned child syscall permission query failed\n");return 9;}
    result=emit(buffer,size,decision);memset(buffer,0,capacity);free(buffer);
    if(result){fprintf(stderr,"unsupported or malformed kernel startup metadata\n");return 8;}
    return 0;
}

// SPDX-License-Identifier: GPL-2.0-only OR BSD-2-Clause

/*
 * Copyright (C) 2021 Elasticsearch BV
 *
 * This software is dual-licensed under the BSD 2-Clause and GPL v2 licenses.
 * You may choose either one of them if you use this software.
 */

#ifndef EBPF_EVENTPROBE_EBPFEVENTPROTO_H
#define EBPF_EVENTPROBE_EBPFEVENTPROTO_H

#define ARGV_MAX 8192 // See issue #43, quite possibly too small

#define PATH_MAX 4096
// When computing the path we need to allocate twice the size of PATH_MAX
// because the verifier does not have a way to know if the path actually
// fits in PATH_MAX
#define PATH_MAX_BUF PATH_MAX * 2

#define TASK_COMM_LEN 16

#define TTY_OUT_MAX 4096

#ifndef __KERNEL__
#include <stdint.h>
#else
#include "vmlinux.h"
#endif

enum ebpf_event_type {
    EBPF_EVENT_PROCESS_FORK                 = (1 << 1),
    EBPF_EVENT_PROCESS_EXEC                 = (1 << 2),
    EBPF_EVENT_PROCESS_EXIT                 = (1 << 3),
    EBPF_EVENT_PROCESS_SETSID               = (1 << 4),
    EBPF_EVENT_PROCESS_SETUID               = (1 << 5),
    EBPF_EVENT_PROCESS_SETGID               = (1 << 6),
    EBPF_EVENT_PROCESS_TTY_WRITE            = (1 << 7),
    EBPF_EVENT_FILE_DELETE                  = (1 << 8),
    EBPF_EVENT_FILE_CREATE                  = (1 << 9),
    EBPF_EVENT_FILE_RENAME                  = (1 << 10),
    EBPF_EVENT_NETWORK_CONNECTION_ACCEPTED  = (1 << 11),
    EBPF_EVENT_NETWORK_CONNECTION_ATTEMPTED = (1 << 12),
    EBPF_EVENT_NETWORK_CONNECTION_CLOSED    = (1 << 13),
};

struct ebpf_event_header {
    uint64_t ts;
    uint64_t type;
} __attribute__((packed));

struct ebpf_pid_info {
    uint64_t start_time_ns;
    uint32_t tid;
    uint32_t tgid;
    uint32_t ppid;
    uint32_t pgid;
    uint32_t sid;
} __attribute__((packed));

struct ebpf_cred_info {
    uint32_t ruid; // Real user ID
    uint32_t rgid; // Real group ID
    uint32_t euid; // Effective user ID
    uint32_t egid; // Effective group ID
    uint32_t suid; // Saved user ID
    uint32_t sgid; // Saved group ID
} __attribute__((packed));

struct ebpf_tty_dev {
    uint16_t minor;
    uint16_t major;
} __attribute__((packed));

// Full events follow
struct ebpf_file_delete_event {
    struct ebpf_event_header hdr;
    struct ebpf_pid_info pids;
    char path[PATH_MAX_BUF];
    uint32_t mntns;
    char comm[TASK_COMM_LEN];
} __attribute__((packed));

struct ebpf_file_create_event {
    struct ebpf_event_header hdr;
    struct ebpf_pid_info pids;
    char path[PATH_MAX_BUF];
    uint32_t mntns;
    char comm[TASK_COMM_LEN];
} __attribute__((packed));

struct ebpf_file_rename_event {
    struct ebpf_event_header hdr;
    struct ebpf_pid_info pids;
    char old_path[PATH_MAX_BUF];
    char new_path[PATH_MAX_BUF];
    uint32_t mntns;
    char comm[TASK_COMM_LEN];
} __attribute__((packed));

struct ebpf_process_fork_event {
    struct ebpf_event_header hdr;
    struct ebpf_pid_info parent_pids;
    struct ebpf_pid_info child_pids;
    char pids_ss_cgroup_path[PATH_MAX];
} __attribute__((packed));

struct ebpf_process_exec_event {
    struct ebpf_event_header hdr;
    struct ebpf_pid_info pids;
    struct ebpf_cred_info creds;
    struct ebpf_tty_dev ctty;
    char filename[PATH_MAX];
    char cwd[PATH_MAX];
    char argv[ARGV_MAX];
    char pids_ss_cgroup_path[PATH_MAX];
} __attribute__((packed));

struct ebpf_process_exit_event {
    struct ebpf_event_header hdr;
    struct ebpf_pid_info pids;
    int32_t exit_code;
    char pids_ss_cgroup_path[PATH_MAX];
} __attribute__((packed));

struct ebpf_process_setsid_event {
    struct ebpf_event_header hdr;
    struct ebpf_pid_info pids;
} __attribute__((packed));

struct ebpf_process_setuid_event {
    struct ebpf_event_header hdr;
    struct ebpf_pid_info pids;
    uint32_t new_ruid;
    uint32_t new_euid;
    uint32_t new_rgid;
    uint32_t new_egid;
} __attribute__((packed));

struct ebpf_process_tty_write_event {
    struct ebpf_event_header hdr;
    struct ebpf_pid_info pids;
    char tty_out[TTY_OUT_MAX];
    uint64_t tty_out_len;
    uint8_t tty_out_truncated;
} __attribute__((packed));

struct ebpf_process_setgid_event {
    struct ebpf_event_header hdr;
    struct ebpf_pid_info pids;
    uint32_t new_rgid;
    uint32_t new_egid;
    uint32_t new_ruid;
    uint32_t new_euid;
} __attribute__((packed));

enum ebpf_net_info_transport {
    EBPF_NETWORK_EVENT_TRANSPORT_TCP = 1,
};

enum ebpf_net_info_af {
    EBPF_NETWORK_EVENT_AF_INET  = 1,
    EBPF_NETWORK_EVENT_AF_INET6 = 2,
};

struct ebpf_net_info_tcp_close {
    uint64_t bytes_sent;
    uint64_t bytes_received;
} __attribute__((packed));

struct ebpf_net_info {
    enum ebpf_net_info_transport transport;
    enum ebpf_net_info_af family;
    union {
        uint8_t saddr[4];
        uint8_t saddr6[16];
    }; // Network byte order
    union {
        uint8_t daddr[4];
        uint8_t daddr6[16];
    };              // Network byte order
    uint16_t sport; // Host byte order
    uint16_t dport; // Host byte order
    uint32_t netns;
    union {
        struct ebpf_net_info_tcp_close close;
    } tcp;
} __attribute__((packed));

struct ebpf_net_event {
    struct ebpf_event_header hdr;
    struct ebpf_pid_info pids;
    struct ebpf_net_info net;
    char comm[TASK_COMM_LEN];
} __attribute__((packed));

#endif // EBPF_EVENTPROBE_EBPFEVENTPROTO_H

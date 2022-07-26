// build +linux

/*
Copyright © 2022 David Guadalupe <guadalupe.david@gmail.com>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package goprocfs

import (
	"fmt"
	"encoding/json"
	"strconv"
	"strings"
	"sort"
	"os"
	"os/user"
	"path/filepath"
	"io/ioutil"
	// "regexp"
	"sync"
	"runtime"
	"unsafe"
	"golang.org/x/sys/unix"
)

var (
	// reStat *regexp.Regexp
	stProcStat string
)

func init() {
	// reStat = regexp.MustCompile(`(?m)^(?P<pid>\d+) \((?P<comm>.+)\) (?P<fields>.*)$`)
	stProcStat = "{" +
		"Pid: %d, " +
		"Comm: %s, " +
		"State: %s, " +
		"Ppid: %d, " +
		"Pgrp: %d, " +
		"Session: %d, " +
		"TtyNr: %d, " +
		"TPGid: %d, " +
		"Flags: %d, " +
		"MinFlt: %d, " +
		"CMinFlt: %d, " +
		"MajFlt: %d, " +
		"CMajFlt: %d, " +
		"UTime: %d, " +
		"STime: %d, " +
		"CUTime: %d, " +
		"CSTime: %d, " +
		"Priority: %d, " +
		"Nice: %d, " +
		"NumThreads: %d, " +
		"ITRealValue: %d, " +
		"StartTime: %d, " +
		"VSize: %d, " +
		"Rss: %d, " +
		"RssLim: %d, " +
		"StartCode: %d, " +
		"EndCode: %d, " +
		"StartStack: %d, " +
		"KStkESP: %d, " +
		"KStkEIP: %d, " +
		"Signal: %d, " +
		"Blocked: %d, " +
		"SigIgnore: %d, " +
		"SigCatch: %d, " +
		"WChan: %d, " +
		"NSwap: %d, " +
		"CNSwap: %d, " +
		"ExitSignal: %d, " +
		"Processor: %d, " +
		"RTPrio: %d, " +
		"Policy: %d, " +
		"DelayAcctBlkIOTicks: %d, " +
		"GuestTime: %d, " +
		"CGuestTime: %d, " +
		"StartData: %d, " +
		"EndData: %d, " +
		"StartBrk: %d, " +
		"ArgStart: %d, " +
		"ArgEnd: %d, " +
		"EnvStart: %d, " +
		"EnvEnd: %d, " +
		"ExitCode: %d" +
		"}"
}

func GetResource(pid int, rc string) ([]byte, error) {
	return ioutil.ReadFile(fmt.Sprintf("/proc/%d/%s", pid, rc))
}

func GetStat(path string) (stat unix.Stat_t, err error) {
	err = unix.Stat(path, &stat)
	return
}

func GetUid(pid int) int {
	// var stat unix.Stat_t
	// if unix.Stat(fmt.Sprintf("/proc/%d", pid), &stat) == nil {
	//   return int(stat.Uid)
	// }
	if stat, err := GetStat(fmt.Sprintf("/proc/%d", pid)); err == nil {
		return int(stat.Uid)
	}
	return -1
}

func GetUser(uid int) (*user.User, error) {
	return user.LookupId(strconv.Itoa(uid))
}

func GetCgroup(pid int) (cgroup string, err error) {
	if data, err := GetResource(pid, "cgroup"); err == nil {
		cgroup = strings.TrimSpace(string(data))
	}
	return
}

func GetOomScoreAdj(pid int) (score int, err error) {
	if data, err := GetResource(pid, "oom_score_adj"); err == nil {
		return strconv.Atoi(strings.TrimSpace(string(data)))
	}
	return
}

type SchedulingPolicy struct {
	Class map[int]string
	NeedPriority []int
	NeedCredentials []int
	Low int
	High int
	None int
}

var IO SchedulingPolicy = SchedulingPolicy{
	Class: map[int]string{
		0: "none",
		1: "realtime",
		2: "best-effort",
		3: "idle",
	},
	NeedPriority: []int{1, 2},
	NeedCredentials: []int{1},
	Low: 7,
	High: 0,
	None: 4,
}

const (
	IOPRIO_CLASS_NONE = iota
	IOPRIO_CLASS_RT
	IOPRIO_CLASS_BE
	IOPRIO_CLASS_IDLE
)

const (
	_ = iota
	IOPRIO_WHO_PROCESS
	IOPRIO_WHO_PGRP
	IOPRIO_WHO_USER
)

const IOPRIO_CLASS_SHIFT = 13

const (
	NONE = iota
	REALTIME
	BEST_EFFORT
	IDLE
)

func IOPrio_Get(pid int) (int, error) {
	ioprio, _, err := unix.Syscall(
		unix.SYS_IOPRIO_GET, IOPRIO_WHO_PROCESS, uintptr(pid), 0,
	)
	if err == 0 {
		return int(ioprio), nil
	}
	return -1, err
}

func IOPrio_Split(ioprio int, class, data *int) {
	// From https://www.kernel.org/doc/html/latest/block/ioprio.html
	*class = ioprio >> IOPRIO_CLASS_SHIFT
	*data = ioprio & 0xff
}

const (
	SCHED_OTHER = iota
	SCHED_FIFO
	SCHED_RR
	SCHED_BATCH
	SCHED_ISO
	SCHED_IDLE
	SCHED_DEADLINE
)

var CPU SchedulingPolicy = SchedulingPolicy{
	Class: map[int]string{
		0: "SCHED_OTHER",
		1: "SCHED_FIFO",
		2: "SCHED_RR",
		3: "SCHED_BATCH",
		// 4: "SCHED_ISO", // Reserved but not implemented yet in linux
		5: "SCHED_IDLE",
		6: "SCHED_DEADLINE",
	},
	NeedPriority: []int{1, 2},
	NeedCredentials: []int{1, 2},
	Low: 1,
	High: 99,
	None: 0,
}

type Sched_Param struct {
	Sched_Priority int
}

func Sched_GetScheduler(pid int) (int, error) {
	class, _, err := unix.Syscall(unix.SYS_SCHED_GETSCHEDULER, uintptr(pid), 0, 0)
	if err == 0 {
		return int(class), nil
	}
	return -1, err
}

func Sched_GetParam(pid int) (int, error) {
	param := Sched_Param{}
	_, _, err := unix.Syscall(
		unix.SYS_SCHED_GETPARAM, uintptr(pid), uintptr(unsafe.Pointer(&param)), 0,
	)
	if err == 0 {
		return param.Sched_Priority, nil
	}
	return -1, err
}

// % cat /proc/$(pidof nvim)/stat
// 14066 (nvim) S 14064 14063 14063 0 -1 4194304 5898 6028 495 394 487 64 88 68 39 19 1 0 1256778 18685952 2655 4294967295 4620288 7319624 3219630688 0 0 0 0 2 536891909 1 0 0 17 0 0 0 0 0 0 8366744 8490776 38150144 3219638342 3219638506 3219638506 3219644398 0

type ProcStat struct {
	stat string			`json:"-"`
	Pid int					`json:"pid"`						// (1) %d *
	Comm string			`json:"comm"`						// (2) %s *
	State string		`json:"state"`					// (3) %c *
	Ppid int				`json:"ppid"`						// (4) %d *
	Pgrp int				`json:"pgrp"`						// (5) %d *
	Session int			`json:"session"`				// (6) %d
	TtyNr int				`json:"tty_nr"`					// (7) %d
	TPGid int				`json:"tpgid"`					// (8) %d
	Flags uint			`json:"flags"`					// (9) %u
	MinFlt uint			`json:"minflt"`					// (10) %lu
	CMinFlt uint		`json:"cminflt"`				// (11) %lu
	MajFlt uint			`json:"majflt"`					// (12) %lu
	CMajFlt uint		`json:"cmajflt"`				// (13) %lu
	UTime uint			`json:"utime"`					// (14) %lu
	STime uint			`json:"stime"`					// (15) %lu
	CUTime int			`json:"cutime"`					// (16) %ld
	CSTime int			`json:"cstime"`					// (17) %ld
	Priority int		`json:"priority"`				// (18) %ld *
	Nice int				`json:"nice"`						// (19) %ld *
	NumThreads int	`json:"num_threads"`		// (20) %ld *
	ITRealValue int	`json:"itrealvalue"`		// (21) %ld
	StartTime uint64	`json:"starttime"`		// (22) %llu
	VSize uint			`json:"vsize"`					// (23) %lu
	Rss int					`json:"rss"`						// (24) %ld
	RssLim uint			`json:"rsslim"`					// (25) %lu
	StartCode uint	`json:"startcode"`			// (26) %lu
	EndCode uint		`json:"endcode"`				// (27) %lu
	StartStack uint	`json:"startstack"`			// (28) %lu
	KStkESP uint		`json:"kstkesp"`				// (29) %lu
	KStkEIP uint		`json:"kstkeip"`				// (30) %lu
	Signal uint			`json:"signal"`					// (31) %lu
	Blocked uint		`json:"blocked"`				// (32) %lu
	SigIgnore uint	`json:"sigignore"`			// (33) %lu
	SigCatch uint		`json:"sigcatch"`				// (34) %lu
	WChan uint			`json:"wchan"`					// (35) %lu
	NSwap uint			`json:"nswap"`					// (36) %lu -
	CNSwap uint			`json:"cnswap"`					// (37) %lu -
	ExitSignal int	`json:"exit_signal"`		// (38) %d
	Processor int		`json:"processor"`			// (39) %d
	RTPrio int			`json:"rtprio"`					// (40) %u *
	Policy int			`json:"policy"`					// (41) %u *
	DelayAcctBlkIOTicks uint64	`json:"delayacct_blkio_ticks"`	// (42) %llu
	GuestTime uint	`json:"guest_time"`			// (43) %lu
	CGuestTime int	`json:"cguest_time"`		// ((44) %ld
	StartData uint	`json:"start_data"`			// (45) %lu
	EndData uint		`json:"end_data"`				// (46) %lu
	StartBrk uint		`json:"start_brk"`			// (47) %lu
	ArgStart uint		`json:"arg_start"`			// (48) %lu
	ArgEnd uint			`json:"arg_end"`				// (49) %lu
	EnvStart uint		`json:"env_start"`			// (50) %lu
	EnvEnd uint			`json:"env_end"`				// (51) %lu
	ExitCode int		`json:"exit_code"`			// (52) %d
}

func (stat *ProcStat) Load(buffer string) (err error) {
	stat.stat = buffer
	var comm string
	// parse
	_, err = fmt.Sscan(
		buffer,
		&stat.Pid,				// (1) %d *
		&comm,
		// &stat.Comm,				// (2) %s *
		&stat.State,			// (3) %c *
		&stat.Ppid,				// (4) %d *
		&stat.Pgrp,				// (5) %d *
		&stat.Session,		// (6) %d
		&stat.TtyNr,			// (7) %d
		&stat.TPGid,			// (8) %d
		&stat.Flags,			// (9) %u
		&stat.MinFlt,			// (10) %lu
		&stat.CMinFlt,		// (11) %lu
		&stat.MajFlt,			// (12) %lu
		&stat.CMajFlt,		// (13) %lu
		&stat.UTime,			// (14) %lu
		&stat.STime,			// (15) %lu
		&stat.CUTime,			// (16) %ld
		&stat.CSTime,			// (17) %ld
		&stat.Priority,		// (18) %ld *
		&stat.Nice,				// (19) %ld *
		&stat.NumThreads,	// (20) %ld *
		&stat.ITRealValue,	// (21) %ld
		&stat.StartTime,	// (22) %llu
		&stat.VSize,			// (23) %lu
		&stat.Rss,				// (24) %ld
		&stat.RssLim,			// (25) %lu
		&stat.StartCode,	// (26) %lu
		&stat.EndCode,		// (27) %lu
		&stat.StartStack,	// (28) %lu
		&stat.KStkESP,		// (29) %lu
		&stat.KStkEIP,		// (30) %lu
		&stat.Signal,			// (31) %lu
		&stat.Blocked,		// (32) %lu
		&stat.SigIgnore,	// (33) %lu
		&stat.SigCatch,		// (34) %lu
		&stat.WChan,			// (35) %lu
		&stat.NSwap,			// (36) %lu -
		&stat.CNSwap,			// (37) %lu -
		&stat.ExitSignal,	// (38) %d
		&stat.Processor,	// (39) %d
		&stat.RTPrio,			// (40) %u *
		&stat.Policy,			// (41) %u *
		&stat.DelayAcctBlkIOTicks,	// (42) %llu
		&stat.GuestTime,	// (43) %lu
		&stat.CGuestTime,	// ((44) %ld
		&stat.StartData,	// (45) %lu
		&stat.EndData,		// (46) %lu
		&stat.StartBrk,		// (47) %lu
		&stat.ArgStart,		// (48) %lu
		&stat.ArgEnd,			// (49) %lu
		&stat.EnvStart,		// (50) %lu
		&stat.EnvEnd,			// (51) %lu
		&stat.ExitCode,		// (52) %d
	)
	stat.Comm = strings.Trim(comm, "()")
	return
}

// func (stat *ProcStat) parseFields(submatch string) (err error) {
//   var (
//     nums = [...]int{0, 1, 2, 15, 16, 17, 37, 38}
//     input []string
//   )
//   s := strings.Fields(submatch)
//   for _, pos := range nums {
//     input = append(input, s[pos])
//   }
//   _, err = fmt.Sscanf(
//     strings.Join(input, ` `),
//     "%s %d %d %d %d %d %d %d",
//     &stat.State,
//     &stat.Ppid,
//     &stat.Pgrp,
//     &stat.Priority,
//     &stat.Nice,
//     &stat.NumThreads,
//     &stat.RTPrio,
//     &stat.Policy,
//   )
//   return
// }

// func (stat *ProcStat) Load(buffer string) (err error) {
//   stat.stat = buffer
//   // parse
//   matches := reStat.FindStringSubmatch(buffer)
//   // check pid value
//   if value, err := strconv.Atoi(matches[1]); err == nil {
//     stat.Pid = value
//     stat.Comm = matches[2]
//     err = stat.parseFields(matches[3])
//   }
//   return
// }

func (stat *ProcStat) Read(pid int) (err error) {
	// read stat data for pid
	if data, err := GetResource(pid, "stat"); err == nil {
		// load
		err = stat.Load(string(data))
	}
	return
}

func (stat *ProcStat) GoString() string {
	return "ProcStat" + stat.String()
}

func (stat *ProcStat) String() string {
	return fmt.Sprintf(
		stProcStat,
		stat.Pid,
		stat.Comm,
		stat.State,
		stat.Ppid,
		stat.Pgrp,
		stat.Session,
		stat.TtyNr,
		stat.TPGid,
		stat.Flags,
		stat.MinFlt,
		stat.CMinFlt,
		stat.MajFlt,
		stat.CMajFlt,
		stat.UTime,
		stat.STime,
		stat.CUTime,
		stat.CSTime,
		stat.Priority,
		stat.Nice,
		stat.NumThreads,
		stat.ITRealValue,
		stat.StartTime,
		stat.VSize,
		stat.Rss,
		stat.RssLim,
		stat.StartCode,
		stat.EndCode,
		stat.StartStack,
		stat.KStkESP,
		stat.KStkEIP,
		stat.Signal,
		stat.Blocked,
		stat.SigIgnore,
		stat.SigCatch,
		stat.WChan,
		stat.NSwap,
		stat.CNSwap,
		stat.ExitSignal,
		stat.Processor,
		stat.RTPrio,
		stat.Policy,
		stat.DelayAcctBlkIOTicks,
		stat.GuestTime,
		stat.CGuestTime,
		stat.StartData,
		stat.EndData,
		stat.StartBrk,
		stat.ArgStart,
		stat.ArgEnd,
		stat.EnvStart,
		stat.EnvEnd,
		stat.ExitCode,
	)
}

type Proc struct {
	ProcStat
	Uid int						`json:"uid"`
	owner user.User	`json:"-"`
	Cgroup [3]string	`json:"cgroup"`
	OomScoreAdj int		`json:"oom_score_adj"`
	IOPrioClass int		`json:"ioprio_class"`
	IOPrioData int		`json:"ionice"`
}

func (p *Proc) setUser() (err error) {
	p.Uid = GetUid(p.Pid)
	if owner, err := GetUser(p.Uid); err == nil {
		p.owner = *owner
	}
	return
}

func (p *Proc) setCgroup() (err error) {
	if cgroup, err := GetCgroup(p.Pid); err == nil {
		if cgroup != "0::/" {
			parts := strings.Split(cgroup, `/`)
			p.Cgroup = [3]string{cgroup, parts[1], parts[len(parts) - 1]}
		} else {
			p.Cgroup = [3]string{"0::/", ``, ``}
		}
	}
	return
}

func (p *Proc) setOomScoreAdj() (err error) {
	if scoreadj, err := GetOomScoreAdj(p.Pid); err == nil {
		p.OomScoreAdj = scoreadj
	}
	return
}

func (p *Proc) setIOPrio() (err error) {
	if ioprio, err := IOPrio_Get(p.Pid); err == nil {
		IOPrio_Split(ioprio, &p.IOPrioClass, &p.IOPrioData)
	}
	return
}

type setter = func() error

func (p *Proc) setters() []setter {
	return []setter{p.setUser, p.setCgroup, p.setOomScoreAdj, p.setIOPrio}
}

func NewProc(pid int) *Proc {
	p := &Proc{ProcStat: ProcStat{Pid: pid}}
	if err:= p.ProcStat.Read(pid); err != nil {
		panic(err)
	}
	for _, function := range p.setters() {
		if err := function(); err != nil {
			panic(err)
		}
	}
	return p
}

func GetCalling() *Proc {
	return NewProc(os.Getpid())
}

func NewProcFromStat(stat string) (p *Proc, err error) {
	p = new(Proc)
	// Stat
	err = p.ProcStat.Load(stat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	for _, function := range p.setters() {
		if err = function(); err != nil {
			break
		}
	}
	return
}

func (p *Proc) GoString() string {
	return "Proc" + fmt.Sprintf(
		"{ProcStat: %s, Uid: %v, owner: %+v, Cgroup: %v, RTPrio: %v, Policy: %v, OomScoreAdj: %v, IOPrioData: %v, IOPrioClass: %v}",
		p.ProcStat.GoString(), p.Uid, p.owner, p.Cgroup, p.RTPrio, p.Policy, p.OomScoreAdj, p.IOPrioData, p.IOPrioClass,
	)
}

func (p *Proc) String() string {
	return fmt.Sprintf(
		"{ProcStat: %s, Uid: %v, owner: %+v, Cgroup: %v, RTPrio: %v, Policy: %v, OomScoreAdj: %v, IOPrioData: %v, IOPrioClass: %v}",
		p.ProcStat.String(), p.Uid, p.owner, p.Cgroup, p.RTPrio, p.Policy, p.OomScoreAdj, p.IOPrioData, p.IOPrioClass,
	)
}

func (p Proc) Json() (result string) {
	if data, err := json.Marshal(p); err != nil {
		panic(err)
	} else {
		result = string(data)
	}
	return
}

func (p Proc) Raw() string {
	return strings.TrimSuffix(
		fmt.Sprintln(
			p.Pid,
			p.Ppid,
			p.Pgrp,
			p.Uid,
			p.Username(),
			p.State,
			p.Comm,
			p.Cgroup[0],
			p.Priority,
			p.Nice,
			p.NumThreads,
			p.RTPrio,
			p.Policy,
			p.OomScoreAdj,
			p.IOPrioClass,
			p.IOPrioData,
		),
		"\n",
	)
}

func (p *Proc) Sched() string {
	return map[int]string{
		0: "other",
		1: "fifo",
		2: "rr",
		3: "batch",
		// 4: "iso", // Reserved but not implemented yet in linux
		5: "idle",
		6: "deadline",
	}[p.Policy]
}

func (p *Proc) CPUSchedInfo() string {
	return fmt.Sprintf(
		"%d:%s:%d", p.Policy, p.Sched(), p.RTPrio,
	)
}

func (p *Proc) IOClass() string {
	return IO.Class[p.IOPrioClass]
}

func (p *Proc) IOSchedInfo() string {
	return fmt.Sprintf(
		"%d:%s:%d", p.IOPrioClass, p.IOClass(), p.IOPrioData,
	)
}

func (p *Proc) Values() string {
	return fmt.Sprintf("[%d,%d,%d,%d,%q,%q,%q,%q,%q,%q,%d,%d,%d,%d,%d,%d,%q,%d]",
		p.Pid,
		p.Ppid,
		p.Pgrp,
		p.Uid,
		p.Username(),
		p.State,
		p.Cgroup[1],
		p.Cgroup[2],
		p.Comm,
		p.Cgroup[0],
		p.Priority,
		p.Nice,
		p.NumThreads,
		p.RTPrio,
		p.Policy,
		p.OomScoreAdj,
		p.IOClass(),
		p.IOPrioData,
	)
}

func (p *Proc) GetStringMap() (result map[string]interface{}) {
	if data, err := json.Marshal(*p); err != nil {
		panic(err)
	} else if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return
}

func (p *Proc) Username() string {
	return p.owner.Username
}

func (p *Proc) InUserSlice() bool {
	return p.Cgroup[1] == "user.slice"
}

func (p *Proc) InSystemSlice() bool {
	return !(p.InUserSlice())
}

func Stat(stat string) (result string) {
	if p, err := NewProcFromStat(stat); err != nil {
		panic(err)
	} else {
		result = p.Values()
	}
	return
}

type ProcFilter struct {
	scope string
	filter func(p *Proc, err error) bool
	message string
}

func (self ProcFilter) Filter(p *Proc, err error) bool {
	return self.filter(p, err)
}

func (self ProcFilter) String() string {
	return self.message
}

type Filterer interface {
	Filter(p *Proc, err error) bool
	String() string
}

func GetFilterer(scope string) ProcFilter {
	switch strings.ToLower(scope) {
	case "global":
		return ProcFilter{
			scope: "global",
			filter: func(p *Proc, err error) bool {
				return err == nil && p.InUserSlice()
			},
			message: "processes inside any user slice",
		}
	case "system":
		return ProcFilter{
			scope: "system",
			filter: func(p *Proc, err error) bool {
				return err == nil && p.InSystemSlice()
			},
			message: "processes inside system slice",
		}
	case "all":
		return ProcFilter{
			scope: "all",
			filter: func(p *Proc, err error) bool {
				return err == nil
			},
			message: "all processes",
		}
	}
	// Default is user
	return ProcFilter{
		scope: "user",
		filter: func(p *Proc, err error) bool {
			return err == nil && p.Uid == os.Getuid() && p.InUserSlice()
		},
		message: "calling user processes",
	}
}

type Formatter func (p *Proc) string

func GetFormatter(format string) Formatter {
	switch strings.ToLower(format) {
	case "json":
		return func(p *Proc) string { return p.Json() }
	case "raw":
		return func(p *Proc) string { return p.Raw() }
	case "values":
		return func(p *Proc) string { return p.Values() }
	default:
		return func(p *Proc) string { return p.String() }
	}
}

// ProcByPid implements sort.Interface for []*Proc based on Pid field
type ProcByPid []*Proc
func (s ProcByPid) Len() int { return len(s) }
func (s ProcByPid) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ProcByPid) Less(i, j int) bool { return s[i].Pid < s[j].Pid }

// FilteredProcs returns a slice of Proc for filtered processes.
func FilteredProcs(filter Filterer) (result []*Proc) {
	files, _ := filepath.Glob("/proc/[0-9]*/stat")
	size := len(files)
	// make our channels for communicating work and results
	stats := make(chan string, size)
	procs := make(chan *Proc, size)
	// spin up workers and use a sync.WaitGroup to indicate completion
	var count = runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(){
			defer wg.Done()
			var p *Proc
			var err error
			for stat := range stats {
				p, err = NewProcFromStat(stat)
				if filter.Filter(p, err) {
					procs <- p
				}
			}
		}()
	}
	// start sending jobs
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, file := range files {
			data, err := ioutil.ReadFile(file)
			if err == nil {
				stats <- string(data)
			}
		}
		close(stats)
	}()
	// wait on the workers to finish and close the procs channel
	// to signal downstream that all work is done
	go func() {
		defer close(procs)
		wg.Wait()
	}()
	// collect result from procs channel
	for p := range procs {
		result = append(result, p)
	}
	// sort by Pid
	sort.Sort(ProcByPid(result))
	return
}

// AllProcs returns a slice of Proc for all processes.
func AllProcs() (result []*Proc) {
	return FilteredProcs(GetFilterer("all"))
}
// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:

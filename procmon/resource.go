package procmon

import (
	"fmt"
	"strconv"
	"strings"
	"bufio"
	"os"
	"path/filepath"
)

var defaultLabels = map[string]string{
	`CpuPercent`: ``,
	`CpuFreq`: ``,
	`LoadAvg`: ``,
	`MemPercent`: ``,
	`SwapUsed`: ``,
	`NetDevice`: ``,
	`DownSpeed`: ``,
	`DownTotal`: ``,
	`UpSpeed`: ``,
	`UpTotal`: ``,
}

type Resource struct {
	Seconds int									`dbus:"-"`
	Name string									`dbus:"-"`
	Statuses map[string]*Status
	kind string									`dbus:"-"`
	path string									`dbus:"-"`
	ordered []string						`dbus:"-"`
	data map[string]int					`dbus:"-"`
	changed map[string]interface{}		`dbus:"-"`
}

func NewResource(name, kind string) *Resource {
	return &Resource{
		Name: name,
		kind: kind,
		Statuses: make(map[string]*Status),
		data: make(map[string]int),
		changed: make(map[string]interface{}),
	}
}

func (rc *Resource) AddStatus(tag string) *Status {
	rc.Statuses[tag] = &Status{Tag: tag, format: "%v"}
	rc.ordered = append(rc.ordered, tag)
	return rc.Statuses[tag]
}

func (rc *Resource) UpdateStatus(s Status) {
	rc.Statuses[s.Tag] = &s
}

func (rc *Resource) SetDefaultLabel(tag string) {
	if label, found := defaultLabels[tag]; found {
		rc.Statuses[tag].SetLabel(label)
	}
}

func (rc *Resource) SetPath(path string) {
	rc.path = path
}

func (rc *Resource) SetData(key string, value int) {
	rc.data[key] = value
}

func (rc *Resource) GetData(key string) int {
	return rc.data[key]
}

func (rc *Resource) ListStatuses() []string {
	return rc.ordered
}

func (rc *Resource) RemoveStatus(tag string) {
	if _, found := rc.Statuses[tag]; found {
		delete(rc.Statuses, tag)
		var buf []string
		for _, v := range rc.ordered {
			if v == tag {
				continue
			}
			buf = append(buf, v)
		}
		rc.ordered = buf
	}
}

func (rc *Resource) OnTagUpdated(tag string, value interface{}) {
	if s, found := rc.Statuses[tag]; found {
		if s.Value != value {
			// fmt.Printf("Changed %s %v\n", tag, value)
			rc.changed[tag] = value
		} else {
			delete(rc.changed, tag)
		}
	}
}

type ResourceBuilder func() *Resource

var Builders map[string]ResourceBuilder = map[string]ResourceBuilder{
	"/proc/stat": func() *Resource {
		rc := NewResource("stat", "file")
		rc.path = "/proc/stat"
		rc.Seconds = 1
		rc.data[`idle`] = 0  // kib
		rc.data[`total`] = 0
		s := rc.AddStatus(`CpuPercent`)
		s.SetFormat("%3d")
		s.AddUnit(NewUnit(1, "%"))
		s.SetLabel(defaultLabels[`CpuPercent`])
		return rc
	},
	"/proc/loadavg": func() *Resource {
		rc := NewResource("loadavg", "file")
		rc.path = "/proc/loadavg"
		rc.Seconds = 5
		s := rc.AddStatus(`LoadAvg`)
		s.SetLabel(defaultLabels[`LoadAvg`])
		return rc
	},
	"/proc/cpuinfo": func() *Resource {
		rc := NewResource("cpuinfo", "file")
		rc.path = "/proc/cpuinfo"
		rc.Seconds = 1
		s := rc.AddStatus(`CpuFreq`)
		s.SetFormat("%4.0f")
		s.AddUnit(NewUnit(1, " MHz"))
		s.SetLabel(defaultLabels[`CpuFreq`])
		return rc
	},
	"/proc/meminfo": func() *Resource {
		rc := NewResource("meminfo", "file")
		rc.path = "/proc/meminfo"
		rc.Seconds = 5
		var s *Status
		s = rc.AddStatus(`MemPercent`)
		s.SetFormat("%3d")
		s.AddUnit(NewUnit(1, "%"))
		s.SetLabel(defaultLabels[`MemPercent`])
		s = rc.AddStatus(`SwapUsed`)
		s.SetFormat("%4d")
		s.AddUnit(NewUnit(1, " Mib"))
		s.SetLabel(defaultLabels[`SwapUsed`])
		return rc
	},
	"/proc/net/dev": func() *Resource {
		rc := NewResource("netdev", "file")
		rc.path = "/proc/net/dev"
		rc.Seconds = 1
		rc.data[`down`] = 0  // bytes
		rc.data[`up`] = 0
		var s *Status
		s = rc.AddStatus(`NetDevice`)
		s.SetLabel(defaultLabels[`NetDevice`])
		s = rc.AddStatus(`DownSpeed`)
		s.SetFormat("%5.1f")
		s.AddUnit(NewUnit(1, " kiB/s"))
		s.AddUnit(NewUnit(1024, " MiB/s"))
		s.SetLabel(defaultLabels[`DownSpeed`])
		s = rc.AddStatus(`DownTotal`)
		s.SetFormat("%5.1f")
		s.AddUnit(NewUnit(1, " MiB"))
		s.AddUnit(NewUnit(1024, " GiB"))
		s.SetLabel(defaultLabels[`DownTotal`])
		s = rc.AddStatus(`UpSpeed`)
		s.SetFormat("%5.1f")
		s.AddUnit(NewUnit(1, " kiB/s"))
		s.AddUnit(NewUnit(1024, " MiB/s"))
		s.SetLabel(defaultLabels[`UpSpeed`])
		s = rc.AddStatus(`UpTotal`)
		s.SetFormat("%5.1f")
		s.AddUnit(NewUnit(1, " MiB"))
		s.AddUnit(NewUnit(1024, " GiB"))
		s.SetLabel(defaultLabels[`UpTotal`])
		return rc
	},
}

func NewFileResource(path string) *Resource {
	if builder, found := Builders[path]; found {
		return builder()
	}
	rc := NewResource(filepath.Base(path), "file")
	rc.path = path
	return rc
}

type ScanHandler func(*bufio.Scanner, *Resource)

var Handlers map[string]ScanHandler = map[string]ScanHandler{
	"/proc/stat": func(scanner *bufio.Scanner, rc *Resource) {
		scanner.Scan() // get first line
		fields := strings.Fields(scanner.Text())
		if scanner.Err() == nil {
			var total, idle, ratio int
			for _, field := range fields[1:] {
				val , _ := strconv.Atoi(field)
				total = total + val
			}
			idle, _ = strconv.Atoi(fields[4])
			ratio = (idle - rc.data[`idle`]) * 100 / (total - rc.data[`total`])
			rc.OnTagUpdated(`CpuPercent`, 100 - ratio)
			rc.data[`idle`] = idle
			rc.data[`total`] = total
		}
	},
	"/proc/loadavg": func(scanner *bufio.Scanner, rc *Resource) {
		scanner.Scan() // get first line
		fields := strings.Fields(scanner.Text())
		if scanner.Err() == nil {
			rc.OnTagUpdated(`LoadAvg`, strings.Join(fields[:3], ` `))
		}
	},
	"/proc/cpuinfo": func(scanner *bufio.Scanner, rc *Resource) {
		var frequency float64
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, `cpu MHz`) {
				fmt.Sscanf(strings.Fields(line)[3], "%f", &frequency)
				break
			}
		}
		if scanner.Err() == nil {
			rc.OnTagUpdated(`CpuFreq`, frequency)
		}
	},
	"/proc/meminfo": func(scanner *bufio.Scanner, rc *Resource) {
		var memTotal, memAvailable, swapTotal, swapFree int
		loop:
			for scanner.Scan() {
				line := scanner.Text()
				switch {
				case strings.HasPrefix(line, `MemTotal`):
					memTotal, _ = strconv.Atoi(strings.Fields(line)[1])
				case strings.HasPrefix(line, `MemAvailable`):
					memAvailable, _ = strconv.Atoi(strings.Fields(line)[1])
					// fmt.Println("MemAvailable:", memAvailable)
				case strings.HasPrefix(line, `SwapTotal`):
					swapTotal, _ = strconv.Atoi(strings.Fields(line)[1])
				case strings.HasPrefix(line, `SwapFree`):
					swapFree, _ = strconv.Atoi(strings.Fields(line)[1])
					break loop // Nothing to read from file after that line
				}
			}
		if scanner.Err() == nil {
			rc.OnTagUpdated(`MemPercent`, (memTotal - memAvailable) * 100 / memTotal)
			rc.OnTagUpdated(`SwapUsed`, (swapTotal - swapFree) / 1024)
		}
	},
	"/proc/net/dev": func(scanner *bufio.Scanner, rc *Resource) {
		var device string
		var down, up int
		for scanner.Scan() {
			tokens := strings.Fields(scanner.Text())
			if tokens[0] != `lo:` && strings.HasSuffix(tokens[0], `:`) {
				device = strings.TrimSuffix(tokens[0], `:`)
				down, _ = strconv.Atoi(tokens[1])
				up, _ = strconv.Atoi(tokens[9])
				break
			}
		}
		if scanner.Err() == nil {
			rc.OnTagUpdated(`NetDevice`, device)
			rc.OnTagUpdated(`DownSpeed`, float64(down - rc.data[`down`]) / 1024.0)
			rc.OnTagUpdated(`DownTotal`, float64(down) / 1048576.0)
			rc.OnTagUpdated(`UpSpeed`, float64(up - rc.data[`up`]) / 1024.0)
			rc.OnTagUpdated(`UpTotal`, float64(up) / 1048576.0)
			rc.data[`down`] = down
			rc.data[`up`] = up
		}
	},
}

func (rc *Resource) RefreshStatuses(output chan<- Status) {
	for tag, value := range rc.changed {
		s := rc.Statuses[tag]
		s.SetValue(value)
		output <- *s
	}
}

func (rc *Resource) FileUpdate() (err error) {
	f, e := os.Open(rc.path)
	if e != nil {
		err = e
		return
	}
	defer f.Close()
	if handler, found := Handlers[rc.path]; found {
		scanner := bufio.NewScanner(f)
		scanner.Split(bufio.ScanLines)
		handler(scanner, rc)
	}
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:

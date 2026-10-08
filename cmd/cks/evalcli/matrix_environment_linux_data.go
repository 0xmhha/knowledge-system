package evalcli

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// This parser is kept platform-neutral so ARM/x86 proc data and constraints
// can be tested on the development host. The observed values are identifiers;
// ARM implementer/part fields are never guessed into a marketing CPU name.
func matrixLinuxCPUIdentity(raw string) string {
	fields := []string{}
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			if strings.TrimSpace(line) == "" && len(fields) > 0 {
				break
			}
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if (key == "model name" || key == "Hardware") && value != "" {
			return value
		}
		switch key {
		case "CPU implementer", "CPU architecture", "CPU variant", "CPU part", "CPU revision":
			if value != "" {
				fields = append(fields, key+"="+value)
			}
		}
	}
	return strings.Join(fields, "; ")
}

func matrixLinuxResourceLimits(read func(string) string) map[string]string {
	limits := map[string]string{}
	for name, path := range map[string]string{
		"v2.cpu.max": "/sys/fs/cgroup/cpu.max", "v2.memory.max": "/sys/fs/cgroup/memory.max", "v2.cpuset.cpus.effective": "/sys/fs/cgroup/cpuset.cpus.effective",
		"v1.cpu.cfs_quota_us": "/sys/fs/cgroup/cpu/cpu.cfs_quota_us", "v1.cpu.cfs_period_us": "/sys/fs/cgroup/cpu/cpu.cfs_period_us",
		"v1.memory.limit_in_bytes": "/sys/fs/cgroup/memory/memory.limit_in_bytes", "v1.cpuset.cpus": "/sys/fs/cgroup/cpuset/cpuset.cpus",
	} {
		if value := read(path); value != "" {
			limits[name] = value
		}
	}
	// A bare host may place this process in a child cgroup. Record self and
	// exposed v2 ancestors as well as the conventional visible mount root.
	for _, line := range strings.Split(read("/proc/self/cgroup"), "\n") {
		if !strings.HasPrefix(line, "0::/") {
			continue
		}
		group := strings.TrimPrefix(line, "0::")
		if strings.Contains(group, "\\") || strings.Contains(group, "/../") || strings.HasSuffix(group, "/..") || filepath.Clean(group) != group {
			continue
		}
		for level := 0; level < 64; level++ {
			prefix := "v2.self."
			if level > 0 {
				prefix = "v2.ancestor" + strconv.Itoa(level) + "."
			}
			for _, name := range []string{"cpu.max", "memory.max", "cpuset.cpus.effective"} {
				if value := read(filepath.Join("/sys/fs/cgroup", group, name)); value != "" {
					limits[prefix+name] = value
				}
			}
			if group == "/" {
				break
			}
			group = filepath.Dir(group)
		}
	}
	return limits
}

func matrixLinuxProcess(pid int, raw string, uptime, ticks float64, pageBytes int64) (matrixProcess, bool) {
	// comm may contain spaces and ')'. The stat suffix consists of numeric/state
	// fields, so use the last closing ')' and discard the entire command name.
	close := strings.LastIndex(raw, ")")
	if close < 0 || ticks <= 0 || pageBytes <= 0 {
		return matrixProcess{}, false
	}
	fields := strings.Fields(raw[close+1:])
	if len(fields) < 22 {
		return matrixProcess{}, false
	}
	parent, e1 := strconv.Atoi(fields[1])
	user, e2 := strconv.ParseFloat(fields[11], 64)
	system, e3 := strconv.ParseFloat(fields[12], 64)
	start, e4 := strconv.ParseFloat(fields[19], 64)
	rss, e5 := strconv.ParseInt(fields[21], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || pid <= 0 || parent < 0 || user < 0 || system < 0 || start < 0 || rss < 0 || rss > math.MaxInt64/pageBytes || math.IsNaN(user) || math.IsNaN(system) || math.IsNaN(start) || math.IsInf(user, 0) || math.IsInf(system, 0) || math.IsInf(start, 0) {
		return matrixProcess{}, false
	}
	elapsed := uptime - start/ticks
	if elapsed <= 0 || math.IsNaN(elapsed) || math.IsInf(elapsed, 0) {
		return matrixProcess{}, false
	}
	cpu := (user + system) / ticks / elapsed * 100
	if math.IsNaN(cpu) || math.IsInf(cpu, 0) {
		return matrixProcess{}, false
	}
	return matrixProcess{PID: pid, ParentPID: parent, CPUPercent: cpu, RSSBytes: rss * pageBytes}, true
}

func matrixLinuxProcesses(root string, read func(string) string, clockTicks string, pageBytes int64) ([]matrixProcess, int) {
	ticks, err := strconv.ParseFloat(clockTicks, 64)
	if err != nil || ticks <= 0 || math.IsNaN(ticks) || math.IsInf(ticks, 0) {
		return nil, 0
	}
	uptimeFields := strings.Fields(read(filepath.Join(root, "uptime")))
	if len(uptimeFields) == 0 {
		return nil, 0
	}
	uptime, err := strconv.ParseFloat(uptimeFields[0], 64)
	if err != nil {
		return nil, 0
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, 0
	}
	all := []matrixProcess{}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		if p, ok := matrixLinuxProcess(pid, read(filepath.Join(root, entry.Name(), "stat")), uptime, ticks, pageBytes); ok {
			all = append(all, p)
		}
	}
	count := len(all)
	sort.Slice(all, func(i, j int) bool {
		if all[i].CPUPercent == all[j].CPUPercent {
			return all[i].PID < all[j].PID
		}
		return all[i].CPUPercent > all[j].CPUPercent
	})
	if len(all) > 8 {
		all = all[:8]
	}
	return all, count
}

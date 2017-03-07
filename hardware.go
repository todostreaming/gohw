/*
This is a package only for Linux x64 hardware (kernel 2.36+)
*/
package gohw

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Status struct {
	CPUName  string
	CPUCores int
	TotalMem uint64
	UsedMem  uint64
	CPUusage float64
	RXbps    uint64
	TXbps    uint64
}

// Objeto hardware
type GoHw struct {
	cpuname  string
	cpucores int    // total num of cores
	totalmem uint64 // total bytes of memory
	iface    string // network interface

	rxbitrate uint64  // received bitrate in bps at iface (latest 10 secs)
	txbitrate uint64  // transmited bitrate in bps at iface (latest 10 secs)
	cpuusage  float64 // cpu usage in %
	usedmem   uint64  // bytes used

	running bool // true/false if GoHw is running or not

	mu sync.Mutex
}

// Constructor del objeto GoHw que analizará el hardware
func Hardware() *GoHw {
	hw := &GoHw{}
	hw.mu.Lock()
	defer hw.mu.Unlock()

	hw.iface = "eth0"
	hw.cpuname = "unknown"
	hw.totalmem = 0
	hw.rxbitrate = 0
	hw.txbitrate = 0
	hw.running = false
	hw.usedmem = 0
	hw.cpuusage = 0.0
	hw.cpucores = 0

	return hw
}

// funcion interna para recoger info del proc stat
func getCPUSample() (idle, total uint64) {
	contents, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] == "cpu" {
			numFields := len(fields)
			for i := 1; i < numFields; i++ {
				val, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					fmt.Println("Error: ", i, fields[i], err)
				}
				total += val // tally up all the numbers to get total ticks
				if i == 4 {  // idle is the 5th field in the cpu line
					idle = val
				}
			}
			return
		}
	}
	return
}

// funcion interna que mide el uso de CPU en % (0-100) hw.cpuusage
func (hw *GoHw) cpumeasure() {
	running := true

	for running {
		time.Sleep(10 * time.Second)
		idle0, total0 := getCPUSample()
		time.Sleep(3 * time.Second)
		idle1, total1 := getCPUSample()
		idleTicks := float64(idle1 - idle0)
		totalTicks := float64(total1 - total0)
		hw.mu.Lock()
		hw.cpuusage = 100 * (totalTicks - idleTicks) / totalTicks
		running = hw.running
		hw.mu.Unlock()
	}
}

// función principal que comienza las mediciones del hardware
func (hw *GoHw) Run(iface string) {
	hw.mu.Lock()
	defer hw.mu.Unlock()

	hw.iface = iface
	hw.running = true
	hw.cpucores = runtime.NumCPU()
	go hw.cpumeasure()
	go hw.getmem()
	go hw.getnetparms(iface)
}

// función que termina las mediciones del hardware (no suele usarse, ya que las mediciones no paran)
func (hw *GoHw) Stop() {
	hw.mu.Lock()
	defer hw.mu.Unlock()

	hw.running = false
}

func (hw *GoHw) getmem() {
	running := true

	for running {
		time.Sleep(10 * time.Second)
		cmd := exec.Command("/bin/sh", "-c", "/usr/bin/free -b | grep -i mem")
		res, err := cmd.CombinedOutput()
		if err != nil {
			hw.mu.Lock()
			running = hw.running
			hw.mu.Unlock()
			continue
		}
		spl := strings.Fields(string(res))
		hw.mu.Lock()
		hw.totalmem = toInt(spl[1])
		hw.usedmem = toInt(spl[2])
		running = hw.running
		hw.mu.Unlock()
	}
}

func (hw *GoHw) getnetparms(iface string) {
	running := true
	var oldrx, oldtx, rx, tx uint64

	for running {
		contents, err := ioutil.ReadFile("/proc/net/dev")
		if err != nil {
			hw.mu.Lock()
			running = hw.running
			hw.mu.Unlock()
			continue
		}
		lines := strings.Split(string(contents), "\n")
		for _, line := range lines {
			if strings.Contains(line, iface) {
				items := strings.Fields(line)
				rx = toInt(items[1])
				tx = toInt(items[9])
			}
		}
		if oldrx > 0 || oldtx > 0 {
			hw.mu.Lock()
			if rx > oldrx {
				hw.rxbitrate = 8 * (rx - oldrx) / 10
			}
			if tx > oldtx {
				hw.txbitrate = 8 * (tx - oldtx) / 10
			}
			hw.mu.Unlock()
		}
		oldrx = rx
		oldtx = tx
		time.Sleep(10 * time.Second)
		hw.mu.Lock()
		running = hw.running
		hw.mu.Unlock()
	}
}

func toInt(cant string) (res uint64) {
	fmt.Sscanf(cant, "%d", &res)
	return
}

// you dont need to call this func less than secondly
func (hw *GoHw) Status() *Status {
	var st Status

	hw.mu.Lock()
	defer hw.mu.Unlock()

	st.CPUCores = hw.cpucores
	st.CPUName = hw.cpuname
	st.CPUusage = hw.cpuusage
	st.RXbps = hw.rxbitrate
	st.TXbps = hw.txbitrate
	st.TotalMem = hw.totalmem
	st.UsedMem = hw.usedmem

	return &st
}

package main

import (
	"fmt"
	"github.com/todostreaming/gohw"
	"time"
)

func main() {
	hw := gohw.Hardware()
	hw.Run("eth0")
	time.Sleep(1 * time.Second)
	st := hw.Status()
	fmt.Printf("CPU: %s (%d cores)\n", st.CPUName, st.CPUCores)
	fmt.Printf("RAM: %d MB\n", st.TotalMem/1024/1000)
	for {
		if st.TotalMem > 0 {
			fmt.Printf("CPU used: %2d%%  RAM used: %2d%%  Rx: %d Kbps   Tx: %d Kbps                        \r", int(st.CPUusage), 100*st.UsedMem/st.TotalMem, st.RXbps/1000, st.TXbps/1000)
		}
		time.Sleep(10 * time.Second)
		st = hw.Status()
	}
}

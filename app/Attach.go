package app

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const (
	timeOut   = 5000
	delayStep = 100
)

type VirtualMachine struct {
	Pid          int32        // 目标进程 PID
	SocketFile   string       // .java_pidXXX 文件
	AttachFile   string       // .attach_pid 文件
	Socket       *Socket      // Socket 连接
	SockFileLock sync.RWMutex // socketFile 的锁，线程安全的将sockPath置为空字符串
}

func NewVirtualMachine(pid int32) *VirtualMachine {
	socketFile := filepath.Join(os.TempDir(), fmt.Sprintf(".java_pid%d", pid))
	attachPath := filepath.Join(os.TempDir(), fmt.Sprintf(".attach_pid%d", pid))
	return &VirtualMachine{
		Pid:        pid,
		SocketFile: socketFile,
		AttachFile: attachPath,
	}
}

// 与目标 JVM 建立连接
func (this *VirtualMachine) Attach() error {
	if !this.existsSocketFile() {
		err := this.createAttachFile()
		if err != nil {
			return err
		}
		err = syscall.Kill(int(this.Pid), syscall.SIGQUIT)
		if err != nil {
			return fmt.Errorf("canot send sigquit to java[%d],%v", this.Pid, err)
		}
		timeSpend := 0
		delay := 0
		// 循环条件：socket文件不存在并且未超时
		// 参考代码open Jdk13下的src/jdk.attach/linux/classes/sun/tools/attach/VirtualMachineImpl.java
		for ; !this.existsSocketFile() && timeSpend <= timeOut; {
			delay += delayStep
			time.Sleep(time.Duration(delay) * time.Millisecond)
			timeSpend += delay
			if timeSpend > timeOut/2 && !this.existsSocketFile() {
				// 最后一次尝试发送SIGQUIT信号给目标JVM
				err = syscall.Kill(int(this.Pid), syscall.SIGQUIT)
				if err != nil {
					return fmt.Errorf("canot send sigquit to java[%d],%v", this.Pid, err)
				}
			}
		}
		if !this.existsSocketFile() {
			return fmt.Errorf("unable to open socket file %s: "+
				"target process %d doesn't respond within %dms "+
				"or HotSpot VM not loaded", this.SocketFile, this.Pid,
				timeSpend)
		}
		// 用完attach_pidXXX就可以删除了，避免占用空间
		this.deleteAttachFile()
	}

	// 监听 connet
	addr, err := net.ResolveUnixAddr("unix", this.SocketFile)
	if err != nil {
		return err
	}

	c, err := net.DialUnix("unix", nil, addr)
	if err != nil {
		return err
	}
	this.Socket = &Socket{c}
	return nil
}

func (this *VirtualMachine) Detach() {
	this.SockFileLock.Lock()
	defer this.SockFileLock.Unlock()
	if this.SocketFile != "" {
		this.SocketFile = ""
	}
}

func (this *VirtualMachine) existsSocketFile() bool {
	_, err := os.Stat(this.SocketFile)
	if err != nil {
		return false
	}
	return true
}

func (this *VirtualMachine) createAttachFile() error {
	f, err := os.Create(this.AttachFile)
	if err != nil {
		return fmt.Errorf("canot create attachFile,%v", err)
	}
	defer f.Close()
	return nil
}

func (this *VirtualMachine) deleteAttachFile() {
	if _, err := os.Stat(this.AttachFile); err != nil {
		return
	}
	err := os.Remove(this.AttachFile)
	if err != nil {
		return
	}
	return
}

func (this *VirtualMachine) LoadAgent(agentJarPath string) error {
	this.SockFileLock.Lock()
	if this.SocketFile == "" {
		return fmt.Errorf("detach function has run")
	}
	this.SockFileLock.Unlock()

	err := this.Socket.Execute("load", "instrument", "false", agentJarPath)
	if err != nil {
		return fmt.Errorf("execute load %s,  %v", agentJarPath, err)
	}
	s, err := this.Socket.ReadString()
	if err != nil {
		return err
	}
	//如果发送 agent jar 包成功，
	if s != "0" {
		return fmt.Errorf("load agent jar err")
	}
	defer func() {
		_ = this.Socket.Close()
	}()
	return nil
}

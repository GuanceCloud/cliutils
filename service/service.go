package service

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"text/template"
)

var (
	initTemplates = map[string]string{

		/////////////////////////////////////////////////////////////////////////////////////////
		// upstart
		/////////////////////////////////////////////////////////////////////////////////////////
		"upstart": `# upstart service settings
description "{{.Description}}"
start on runlevel [2345]
stop on runlevel [!2345]
env HOME=/root
export HOME
respawn
respawn limit 10 5
umask 022
chdir {{.InstallDir}}
exec {{.StartCmd}}
post-stop exec sleep 1`,

		/////////////////////////////////////////////////////////////////////////////////////////
		// systemd
		/////////////////////////////////////////////////////////////////////////////////////////
		`systemd`: `# systemd serivce setting
[Unit]
Description={{.Description}}
After=network.target

[Service]
Environment=HOME=/root
WorkingDirectory={{.InstallDir}}
ExecReload=/bin/kill -2 $MAINPID
KillMode=process
Restart=always
RestartSec=3s
ExecStart={{.StartCmd}}

[Install]
WantedBy=default.target`,
	}

	ErrUnknownInstallType = errors.New(`unknown install type`)
	ErrSystemdPathMissing = errors.New(`systemd path not found`)
)

type Service struct {
	Name        string
	Description string
	InstallDir  string
	StartCMD    string

	upstart string
	systemd string
}

// 根据模板，生成多种启动文件
func (s *Service) genInit() error {
	for k, init := range initTemplates {
		t := template.New(``)
		t, err := t.Parse(init)
		if err != nil {
			return err
		}

		var f string
		switch k {
		case `upstart`:
			f = path.Join(s.InstallDir, `deamon.conf`)
			s.upstart = f
		case `systemd`:
			f = path.Join(s.InstallDir, `deamon.service`)
			s.systemd = f
		default:
			return ErrUnknownInstallType
		}

		fd, err := os.OpenFile(f, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
		if err != nil {
			return err
		}

		defer fd.Close()

		if err := t.Execute(fd, s); err != nil {
			return err
		}
	}

	return nil
}

var (
	upstartStop = `stop`
	systemd     = `systemctl`
)

func detectInitType() string {
	cmds := []string{upstartStop, systemd}

	for _, cmd := range cmds {
		_, err := exec.LookPath(cmd)
		if err == nil {
			return cmd
		}
	}

	return ""
}

func (s *Service) installAndStart() error {
	switch detectInitType() {
	case upstartStop:
		return s.upstartInstall()
	case systemd:
		return s.systemdInstall()
	}

	return ErrUnknownInstallType
}

func (s *Service) upstartInstall() error {
	cmd := exec.Command(`stop`, []string{s.Name}...)
	_, err := cmd.Output()
	if err != nil {
		return err
	}

	data, err := ioutil.ReadFile(s.upstart)
	if err != nil {
		return err
	}

	installPath := path.Join(`/etc/init`, s.Name+`.conf`)
	if err := ioutil.WriteFile(installPath, data, os.ModePerm); err != nil {
		return err
	}

	cmd = exec.Command(`start`, []string{s.Name}...)
	if _, err := cmd.Output(); err != nil {
		return err
	}
	return nil
}

func (s *Service) systemdInstall() error {

	cmd := exec.Command(`sytemctl`, []string{`stop`, s.Name}...)
	_, err := cmd.Output()
	if err != nil {
		return err
	}

	systemdPath := ""
	systemdPaths := []string{`/lib/systemd`, `/etc/systemd`}
	for _, p := range systemdPaths { // 检测可选的安装目录是否存在
		if _, err := os.Stat(p); err == nil {
			systemdPath = p
			break
		}
	}

	if systemdPath == `` {
		return ErrSystemdPathMissing
	}

	i, err := ioutil.ReadFile(s.systemd)
	if err != nil {
		return err
	}

	to := path.Join(systemdPath, `system`, s.Name+`.service`)

	if err := ioutil.WriteFile(to, i, os.ModePerm); err != nil {
		return err
	}

	cmds := []*exec.Cmd{
		exec.Command(`systemctl`, []string{`enable`, s.Name + `.service`}...),
		exec.Command(`systemctl`, []string{`start`, s.Name + `.service`}...),
	}

	for _, cmd := range cmds {
		_, err := cmd.Output()
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) Install() error {
	s.genInit()
	return s.installAndStart()
}

func StopService(name string) error {
	var cmd *exec.Cmd
	switch detectInitType() {
	case upstartStop:
		cmd = exec.Command(`stop`, []string{name}...)

	case systemd:
		cmd = exec.Command(`systemctl`, []string{`stop`, name}...)
	}

	_, err := cmd.Output()

	return err
}

func RestartService(name string) error {
	var cmd *exec.Cmd
	switch detectInitType() {
	case upstartStop:
		cmd = exec.Command(`restart`, []string{name}...)

	case systemd:
		cmd = exec.Command(`systemctl`, []string{`restart`, name}...)
	}

	_, err := cmd.Output()

	return err
}

func StartService(name string) error {
	var cmd *exec.Cmd
	switch detectInitType() {
	case upstartStop:
		cmd = exec.Command(`start`, []string{name}...)

	case systemd:
		cmd = exec.Command(`systemctl`, []string{`start`, name}...)
	}

	_, err := cmd.Output()

	return err
}

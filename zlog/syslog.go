// +build !js !windows

package zlog

import (
	"log/syslog"
)

var SyslogName string

var syslogWriter *syslog.Writer

func writeToSyslog(str string) {
	if SyslogName != "" {
		if syslogWriter == nil {
			syslogWriter, _ = syslog.New(syslog.LOG_NOTICE, SyslogName)
		}
		go syslogWriter.Notice(str)
	}
}

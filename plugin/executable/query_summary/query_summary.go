/*
 * Copyright (C) 2020-2022, IrineSistiana
 *
 * This file is part of mosdns.
 *
 * mosdns is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * mosdns is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package query_summary

import (
	"context"
	"fmt"
	"log/syslog"
	"strings"
	"time"

	"github.com/IrineSistiana/mosdns/v4/coremain"
	"github.com/IrineSistiana/mosdns/v4/pkg/executable_seq"
	"github.com/IrineSistiana/mosdns/v4/pkg/query_context"
	"github.com/miekg/dns"
	"go.uber.org/zap"
)

const (
	PluginType = "query_summary"
)

func init() {
	coremain.RegNewPluginFunc(PluginType, Init, func() interface{} { return new(Args) })
	coremain.RegNewPersetPluginFunc("_query_summary", func(bp *coremain.BP) (coremain.Plugin, error) {
		return newLogger(bp, &Args{}), nil
	})
}

var _ coremain.ExecutablePlugin = (*logger)(nil)

type Args struct {
	EnableLocalLog bool   `yaml:"enable_local_log"` // 是否启用本地日志
	Msg            string `yaml:"msg"`
	// 群晖日志服务配置
	SynologyLog *SynologyLogConfig `yaml:"synology_log"`
}

type SynologyLogConfig struct {
	Enabled  bool   `yaml:"enabled"`  // 是否启用群晖日志
	Host     string `yaml:"host"`     // 群晖 IP 地址
	Port     int    `yaml:"port"`     // Syslog 端口，默认 514
	Protocol string `yaml:"protocol"` // tcp 或 udp，默认 udp
	Tag      string `yaml:"tag"`      // 日志标签，默认 "mosdns"
}

func (a *Args) init() {
	if len(a.Msg) == 0 {
		a.Msg = "query summary"
	}
	if a.SynologyLog != nil && a.SynologyLog.Enabled {
		if a.SynologyLog.Port == 0 {
			a.SynologyLog.Port = 514
		}
		if a.SynologyLog.Protocol == "" {
			a.SynologyLog.Protocol = "udp"
		}
		if a.SynologyLog.Tag == "" {
			a.SynologyLog.Tag = "mosdns"
		}
	}
}

type logger struct {
	args      *Args
	syslogger *syslog.Writer
	logChan   chan string // 异步日志通道
	LocalLog  bool        // 是否启用本地日志
	*coremain.BP
}

// Init is a handler.NewPluginFunc.
func Init(bp *coremain.BP, args interface{}) (p coremain.Plugin, err error) {
	return newLogger(bp, args.(*Args)), nil
}

func newLogger(bp *coremain.BP, args *Args) coremain.Plugin {
	args.init()
	l := &logger{BP: bp, args: args, LocalLog: args.EnableLocalLog}

	// 如果启用了群晖日志，初始化 syslog 连接和异步处理
	if args.SynologyLog != nil && args.SynologyLog.Enabled {
		network := args.SynologyLog.Protocol
		addr := fmt.Sprintf("%s:%d", args.SynologyLog.Host, args.SynologyLog.Port)

		syslogger, err := syslog.Dial(network, addr, syslog.LOG_INFO, args.SynologyLog.Tag)
		if err != nil {
			bp.L().Error("failed to connect to synology syslog", zap.Error(err))
		} else {
			l.syslogger = syslogger
			l.logChan = make(chan string, 1000) // 缓冲通道，避免阻塞
			bp.L().Info("connected to synology syslog", zap.String("addr", addr))

			// 启动异步日志处理 goroutine
			go l.asyncLogHandler()
		}
	}

	return l
}

func (l *logger) Exec(ctx context.Context, qCtx *query_context.Context, next executable_seq.ExecutableChainNode) error {
	err := executable_seq.ExecChainNode(ctx, qCtx, next)

	q := qCtx.Q()
	if len(q.Question) != 1 {
		return nil
	}
	question := q.Question[0]
	respRcode := -1
	var answers []string

	if r := qCtx.R(); r != nil {
		respRcode = r.Rcode

		// 收集解析结果
		for _, rr := range r.Answer {
			switch rr := rr.(type) {
			case *dns.A:
				answers = append(answers, rr.A.String())
			case *dns.AAAA:
				answers = append(answers, rr.AAAA.String())
			case *dns.CNAME:
				answers = append(answers, rr.Target)
			case *dns.MX:
				answers = append(answers, rr.Mx)
			case *dns.NS:
				answers = append(answers, rr.Ns)
			case *dns.PTR:
				answers = append(answers, rr.Ptr)
			case *dns.TXT:
				for _, txt := range rr.Txt {
					answers = append(answers, txt)
				}
			case *dns.SRV:
				answers = append(answers, rr.Target)
			default:
				// 对于其他类型的记录，显示其字符串表示
				answers = append(answers, rr.String())
			}
		}
	}

	if l.LocalLog {
		// 构建日志字段
		logFields := []zap.Field{
			zap.Uint32("uqid", qCtx.Id()),
			zap.String("qname", question.Name),
			zap.Uint16("qtype", question.Qtype),
			zap.Uint16("qclass", question.Qclass),
			zap.Stringer("client", qCtx.ReqMeta().ClientAddr),
			zap.Int("resp_rcode", respRcode),
			zap.Duration("elapsed", time.Since(qCtx.StartTime())),
			zap.Error(err),
		}

		// 如果有解析结果，添加到日志中
		if len(answers) > 0 {
			logFields = append(logFields, zap.Strings("answers", answers))
		}

		l.BP.L().Info(l.args.Msg, logFields...)
	}

	// 如果启用了群晖日志，异步发送到 Syslog
	if l.logChan != nil {
		l.sendToSynologyAsync(qCtx, question, respRcode, answers, err)
	}

	return err
}

// sendToSynologyAsync 异步发送日志到群晖 Syslog 服务
func (l *logger) sendToSynologyAsync(qCtx *query_context.Context, question dns.Question, respRcode int, answers []string, err error) {
	// 构建日志消息
	var logParts []string

	logParts = append(logParts, fmt.Sprintf("uqid=%d", qCtx.Id()))
	logParts = append(logParts, fmt.Sprintf("qname=%s", question.Name))
	logParts = append(logParts, fmt.Sprintf("qtype=%d", question.Qtype))
	logParts = append(logParts, fmt.Sprintf("client=%s", qCtx.ReqMeta().ClientAddr.String()))
	logParts = append(logParts, fmt.Sprintf("rcode=%d", respRcode))
	logParts = append(logParts, fmt.Sprintf("elapsed=%s", time.Since(qCtx.StartTime()).String()))

	if len(answers) > 0 {
		logParts = append(logParts, fmt.Sprintf("answers=%s", strings.Join(answers, ",")))
	}

	if err != nil {
		logParts = append(logParts, fmt.Sprintf("error=%s", err.Error()))
	}

	logMessage := strings.Join(logParts, " ")

	// 非阻塞发送到通道
	select {
	case l.logChan <- logMessage:
		// 成功发送到通道
	default:
		// 通道满了，丢弃这条日志，避免阻塞
		l.BP.L().Warn("synology log channel is full, dropping log message")
	}
}

// asyncLogHandler 异步处理日志发送
func (l *logger) asyncLogHandler() {
	for logMessage := range l.logChan {
		if l.syslogger != nil {
			if err := l.syslogger.Info(logMessage); err != nil {
				l.BP.L().Warn("failed to send log to synology", zap.Error(err))
			}
		}
	}
}

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

package data_provider

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/IrineSistiana/mosdns/v4/pkg/safe_close"
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

type DataManager struct {
	pm sync.RWMutex
	ps map[string]*DataProvider
}

type DataListener interface {
	Update(newData []byte) error
}

func NewDataManager() *DataManager {
	return &DataManager{
		ps: make(map[string]*DataProvider),
	}
}

func (m *DataManager) AddDataProvider(name string, p *DataProvider) {
	m.pm.Lock()
	defer m.pm.Unlock()
	m.ps[name] = p
}

func (m *DataManager) GetDataProvider(name string) *DataProvider {
	m.pm.RLock()
	defer m.pm.RUnlock()
	return m.ps[name]
}

type DataProviderConfig struct {
	Tag        string `yaml:"tag"`
	File       string `yaml:"file"`
	AutoReload bool   `yaml:"auto_reload"`
	Remote     string `yaml:"remote"` // remote data provider, e.g. http://example.com/data.json
}

type DataProvider struct {
	logger     *zap.Logger
	file       string
	autoReload bool

	Remote string // remote data provider, e.g. http://example.com/data.json

	// 添加 ETag 缓存字段
	etag   string
	etagMu sync.RWMutex

	lm        sync.Mutex
	listeners map[DataListener]struct{}

	sc *safe_close.SafeClose
}

func NewDataProvider(lg *zap.Logger, cfg DataProviderConfig) (*DataProvider, error) {
	dp := new(DataProvider)
	dp.logger = lg
	dp.file = cfg.File
	dp.autoReload = cfg.AutoReload
	dp.Remote = cfg.Remote

	dp.sc = safe_close.NewSafeClose()

	if err := dp.init(); err != nil {
		return nil, err
	}
	return dp, nil
}

func (ds *DataProvider) init() error {
	if ds.Remote != "" {
		// 对于远程数据源，优先使用本地文件（如果存在）
		if ds.file != "" {
			if _, err := os.Stat(ds.file); err == nil {
				// 本地文件存在，使用本地文件
				_, err := ds.loadFromDisk()
				if err != nil {
					return err
				}
			} else {
				// 本地文件不存在，从远程加载
				_, err := ds.loadFromRemote()
				if err != nil {
					return err
				}
			}
		} else {
			// 没有指定本地文件，直接从远程加载
			_, err := ds.loadFromRemote()
			if err != nil {
				return err
			}
		}
	} else {
		_, err := ds.loadFromDisk()
		if err != nil {
			return err
		}
	}

	if ds.autoReload {
		if ds.Remote == "" {
			ds.logger.Info(
				"auto reload enabled, will watch file for changes",
				zap.String("file", ds.file),
			)
			if err := ds.startFsWatcher(); err != nil {
				return fmt.Errorf("failed to start fs watcher, %w", err)
			}
		} else {
			ds.logger.Info(
				"auto reload enabled, will watch remote for changes",
				zap.String("remote", ds.Remote),
			)
			if err := ds.startRemoteWatcher(); err != nil {
				return fmt.Errorf("failed to start remote watcher, %w", err)
			}
		}
	}
	return nil
}

func (ds *DataProvider) Close() {
	ds.sc.Done()
	ds.sc.CloseWait()
}

// LoadAndAddListener loads the DataListener, returns any error that occurs, and
// add this DataListener to this DataProvider.
func (ds *DataProvider) LoadAndAddListener(l DataListener) error {
	b, err := ds.GetData()
	if err != nil {
		return err
	}

	if err := l.Update(b); err != nil {
		return err
	}

	ds.lm.Lock()
	if ds.listeners == nil {
		ds.listeners = make(map[DataListener]struct{})
	}
	ds.listeners[l] = struct{}{}
	ds.lm.Unlock()
	return nil
}

func (ds *DataProvider) DeleteListener(l DataListener) {
	ds.lm.Lock()
	defer ds.lm.Unlock()
	delete(ds.listeners, l)
}

func (ds *DataProvider) GetData() ([]byte, error) {
	if ds.Remote != "" {
		return ds.loadFromRemote()
	}
	return os.ReadFile(ds.file)
}

// pushData notify the notifier and trigger all listeners.
func (ds *DataProvider) pushData(newData []byte) {
	ds.lm.Lock()
	ls := make([]DataListener, 0, len(ds.listeners))
	for listener := range ds.listeners {
		ls = append(ls, listener)
	}
	ds.lm.Unlock()

	for _, l := range ls {
		if err := l.Update(newData); err != nil {
			ds.logger.Error(
				"failed to update data listener",
				zap.Error(err),
			)
		}
	}
}

func (ds *DataProvider) loadFromDisk() ([]byte, error) {
	return os.ReadFile(ds.file)
}

func (ds *DataProvider) startFsWatcher() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := w.Add(ds.file); err != nil {
		return err
	}

	go func() {
		defer w.Close()

		var delayReloadTimer *time.Timer
		for {
			select {
			case e, ok := <-w.Events:
				if !ok {
					return
				}
				ds.logger.Info(
					"fs event",
					zap.Stringer("event", e.Op),
					zap.String("file", e.Name),
				)

				if delayReloadTimer != nil {
					delayReloadTimer.Stop()
				}
				delayReloadTimer = time.AfterFunc(time.Second, func() {
					if hasOp(e, fsnotify.Remove) {
						_ = w.Remove(ds.file)
						if err := w.Add(ds.file); err != nil {
							ds.logger.Error(
								"failed to re-watch file, auto reload may not work anymore",
								zap.String("file", ds.file),
								zap.Error(err),
							)
						}
					}

					ds.logger.Info(
						"reloading file",
						zap.String("file", ds.file),
					)
					if v, err := ds.loadFromDisk(); err != nil {
						ds.logger.Error(
							"failed to reload file",
							zap.String("file", ds.file),
							zap.Error(err),
						)
					} else {
						ds.logger.Info(
							"file reloaded",
							zap.String("file", ds.file),
						)
						ds.pushData(v)
					}
				})

			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				ds.logger.Error("fs notify error", zap.Error(err))
			case <-ds.sc.ReceiveCloseSignal():
				return
			}
		}
	}()
	return nil
}

func (ds *DataProvider) loadFromRemote() ([]byte, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", ds.Remote, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 添加 If-None-Match 头部以支持 ETag
	ds.etagMu.RLock()
	if ds.etag != "" {
		req.Header.Set("If-None-Match", ds.etag)
	}
	ds.etagMu.RUnlock()

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		ds.logger.Debug(
			"remote data not modified",
			zap.String("remote", ds.Remote),
			zap.String("etag", ds.etag),
		)
		// 数据未更改，返回当前缓存的数据
		if ds.file != "" {
			return os.ReadFile(ds.file)
		}
		return nil, fmt.Errorf("remote data not modified and no local cache available")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// 更新 ETag
	if newEtag := resp.Header.Get("ETag"); newEtag != "" {
		ds.etagMu.Lock()
		ds.etag = newEtag
		ds.etagMu.Unlock()
	}

	// 如果设置了本地文件路径，则缓存到本地
	if ds.file != "" {
		if err := os.WriteFile(ds.file, data, 0644); err != nil {
			ds.logger.Warn("failed to cache remote data to local file", zap.Error(err))
		}
	}

	return data, nil
}

func (ds *DataProvider) startRemoteWatcher() error {
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ds.logger.Info("checking for remote data updates", zap.String("remote", ds.Remote))

				// 记录检查前的 ETag
				ds.etagMu.RLock()
				oldEtag := ds.etag
				ds.etagMu.RUnlock()

				if v, err := ds.loadFromRemote(); err != nil {
					ds.logger.Error(
						"failed to reload remote data",
						zap.String("remote", ds.Remote),
						zap.Error(err),
					)
				} else {
					// 获取检查后的 ETag
					ds.etagMu.RLock()
					newEtag := ds.etag
					ds.etagMu.RUnlock()

					// 如果是第一次检查（oldEtag为空）或者ETag发生变化，则认为数据已更新
					if oldEtag == "" || oldEtag != newEtag {
						ds.logger.Info(
							"remote data updated",
							zap.String("remote", ds.Remote),
							zap.String("old_etag", oldEtag),
							zap.String("new_etag", newEtag),
						)
						ds.pushData(v)
					} else {
						ds.logger.Debug(
							"remote data not changed",
							zap.String("remote", ds.Remote),
							zap.String("etag", newEtag),
						)
					}
				}

			case <-ds.sc.ReceiveCloseSignal():
				return
			}
		}
	}()
	return nil
}

func hasOp(e fsnotify.Event, op fsnotify.Op) bool {
	return e.Op&op == op
}

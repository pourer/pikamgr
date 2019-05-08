// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/sync2/atomic2"
	redigo "github.com/garyburd/redigo/redis"
)

type Sentinel struct {
	context.Context
	Cancel context.CancelFunc

	Product, Auth string

	LogFunc func(format string, args ...interface{})
	ErrFunc func(format string, args ...interface{})
}

func NewSentinel(product, auth string) *Sentinel {
	s := &Sentinel{Product: product, Auth: auth}
	s.Context, s.Cancel = context.WithCancel(context.Background())
	return s
}

func (s *Sentinel) IsCanceled() bool {
	select {
	case <-s.Context.Done():
		return true
	default:
		return false
	}
}

func (s *Sentinel) NodeName(groupName string) string {
	return fmt.Sprintf("%s-%s", s.Product, groupName)
}

func (s *Sentinel) isSameProduct(name string) (groupName string, _ bool) {
	if !strings.HasPrefix(name, s.Product) {
		return "", false
	}
	var suffix = name[len(s.Product):]
	if len(suffix) <= 1 || suffix[0] != '-' {
		return "", false
	}
	return suffix[1:], true
}

func (s *Sentinel) printf(format string, args ...interface{}) {
	if s.LogFunc != nil {
		s.LogFunc(format, args...)
	}
}

func (s *Sentinel) errorf(format string, args ...interface{}) {
	if s.ErrFunc != nil {
		s.ErrFunc(format, args...)
	}
}

func (s *Sentinel) do(sentinel string, timeout time.Duration,
	fn func(client *Client) error) error {
	c, err := NewClientNoAuth(sentinel, timeout)
	if err != nil {
		return err
	}
	defer c.Close()
	return fn(c)
}

func (s *Sentinel) dispatch(ctx context.Context, sentinel string, timeout time.Duration,
	fn func(client *Client) error) error {
	c, err := NewClientNoAuth(sentinel, timeout)
	if err != nil {
		return err
	}
	defer c.Close()

	var exit = make(chan error, 1)

	go func() {
		exit <- fn(c)
	}()

	select {
	case <-ctx.Done():
		return errors.Trace(ctx.Err())
	case err := <-exit:
		return err
	}
}

func (s *Sentinel) subscribeCommand(client *Client, sentinel string,
	onSubscribed func()) error {
	defer func() {
		client.Close()
	}()
	var channels = []interface{}{"+switch-master"}
	go func() {
		client.Send("SUBSCRIBE", channels...)
		client.Flush()
	}()
	for _, sub := range channels {
		values, err := redigo.Values(client.Receive())
		if err != nil {
			return errors.Trace(err)
		} else if len(values) != 3 {
			return errors.Errorf("invalid response = %v", values)
		}
		s, err := redigo.Strings(values[:2], nil)
		if err != nil || s[0] != "subscribe" || s[1] != sub.(string) {
			return errors.Errorf("invalid response = %v", values)
		}
	}
	onSubscribed()
	for {
		values, err := redigo.Values(client.Receive())
		if err != nil {
			return errors.Trace(err)
		} else if len(values) < 2 {
			return errors.Errorf("invalid response = %v", values)
		}
		message, err := redigo.Strings(values, nil)
		if err != nil || message[0] != "message" {
			return errors.Errorf("invalid response = %v", values)
		}
		s.printf("sentinel-[%s] subscribe event %v", sentinel, message)

		switch message[1] {
		case "+switch-master":
			if len(message) != 3 {
				return errors.Errorf("invalid response = %v", values)
			}
			var params = strings.SplitN(message[2], " ", 2)
			if len(params) != 2 {
				return errors.Errorf("invalid response = %v", values)
			}
			_, yes := s.isSameProduct(params[0])
			if yes {
				return nil
			}
		}
	}
}

func (s *Sentinel) subscribeDispatch(ctx context.Context, sentinel string, timeout time.Duration,
	onSubscribed func()) (bool, error) {
	var err = s.dispatch(ctx, sentinel, timeout, func(c *Client) error {
		return s.subscribeCommand(c, sentinel, onSubscribed)
	})
	if err != nil {
		switch errors.Cause(err) {
		case context.Canceled, context.DeadlineExceeded:
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}

func (s *Sentinel) Subscribe(sentinels []string, timeout time.Duration, onMajoritySubscribed func()) bool {
	cntx, cancel := context.WithTimeout(s.Context, timeout)
	defer cancel()

	timeout += time.Second * 5
	results := make(chan bool, len(sentinels))

	var majority = 1 + len(sentinels)/2

	var subscribed atomic2.Int64
	for i := range sentinels {
		go func(sentinel string) {
			notified, err := s.subscribeDispatch(cntx, sentinel, timeout, func() {
				if subscribed.Incr() == int64(majority) {
					onMajoritySubscribed()
				}
			})
			if err != nil {
				s.errorf("sentinel-[%s] subscribe failed. err:%s", sentinel, err.Error())
			}
			results <- notified
		}(sentinels[i])
	}

	for alive := len(sentinels); ; alive-- {
		if alive < majority {
			if cntx.Err() == nil {
				s.printf("sentinel subscribe lost majority (%d/%d)", alive, len(sentinels))
			}
			return false
		}
		select {
		case <-cntx.Done():
			if cntx.Err() != context.DeadlineExceeded {
				s.printf("sentinel subscribe canceled (%v)", cntx.Err())
			}
			return false
		case notified := <-results:
			if notified {
				s.printf("sentinel subscribe notified +switch-master")
				return true
			}
		}
	}
}

func (s *Sentinel) existsCommand(client *Client, names []string) (map[string]bool, error) {
	defer func() {
		if !client.isRecyclable() {
			client.Close()
		}
	}()

	exists := make(map[string]bool, len(names))
	for _, name := range names {
		r, err := client.Do("SENTINEL", "get-master-addr-by-name", name)
		if err != nil {
			return nil, errors.Trace(err)
		}
		exists[name] = (r != nil)
	}
	return exists, nil
}

func (s *Sentinel) slavesCommand(client *Client, names []string) (map[string][]map[string]string, error) {
	defer func() {
		if !client.isRecyclable() {
			client.Close()
		}
	}()

	exists, err := s.existsCommand(client, names)
	if err != nil {
		return nil, err
	}

	results := make(map[string][]map[string]string, len(names))
	for _, name := range names {
		if !exists[name] {
			continue
		}
		values, err := redigo.Values(client.Do("SENTINEL", "slaves", name))
		if err != nil {
			return nil, errors.Trace(err)
		}
		var slaves []map[string]string
		for i := range values {
			m, err := redigo.StringMap(values[i], nil)
			if err != nil {
				return nil, errors.Trace(err)
			}
			slaves = append(slaves, m)
		}
		results[name] = slaves
	}
	return results, nil
}

func (s *Sentinel) mastersCommand(client *Client) (map[string]map[string]string, error) {
	defer func() {
		if !client.isRecyclable() {
			client.Close()
		}
	}()
	values, err := redigo.Values(client.Do("SENTINEL", "masters"))
	if err != nil {
		return nil, errors.Trace(err)
	}
	var masters = make(map[string]map[string]string)
	for i := range values {
		p, err := redigo.StringMap(values[i], nil)
		if err != nil {
			return nil, errors.Trace(err)
		}
		groupName, yes := s.isSameProduct(p["name"])
		if yes {
			masters[groupName] = p
		}
	}
	return masters, nil
}

func (s *Sentinel) mastersDispatch(ctx context.Context, sentinel string, timeout time.Duration) (map[string]*SentinelMaster, error) {
	var masters = make(map[string]*SentinelMaster)
	var err = s.dispatch(ctx, sentinel, timeout, func(c *Client) error {
		p, err := s.mastersCommand(c)
		if err != nil {
			return err
		}
		for groupName, master := range p {
			epoch, err := strconv.ParseInt(master["config-epoch"], 10, 64)
			if err != nil {
				s.printf("sentinel-[%s] masters parse %s failed, config-epoch = '%s', %s",
					sentinel, master["name"], master["config-epoch"], err)
				continue
			}
			var ip, port = master["ip"], master["port"]
			if ip == "" || port == "" {
				s.printf("sentinel-[%s] masters parse %s failed, ip:port = '%s:%s'",
					sentinel, master["name"], ip, port)
				continue
			}
			masters[groupName] = &SentinelMaster{
				Addr: net.JoinHostPort(ip, port),
				Info: master, Epoch: epoch,
			}
		}
		return nil
	})
	if err != nil {
		switch errors.Cause(err) {
		case context.Canceled:
			return nil, nil
		default:
			return nil, err
		}
	}
	return masters, nil
}

type SentinelMaster struct {
	Addr  string
	Info  map[string]string
	Epoch int64
}

func (s *Sentinel) Masters(sentinels []string, timeout time.Duration) (map[string]string, error) {
	cntx, cancel := context.WithTimeout(s.Context, timeout)
	defer cancel()

	timeout += time.Second * 5
	results := make(chan map[string]*SentinelMaster, len(sentinels))

	var majority = 1 + len(sentinels)/2

	for i := range sentinels {
		go func(sentinel string) {
			masters, err := s.mastersDispatch(cntx, sentinel, timeout)
			if err != nil {
				s.errorf("sentinel-[%s] masters failed. err:%s", sentinel, err.Error())
			}
			results <- masters
		}(sentinels[i])
	}

	masters := make(map[string]string)
	current := make(map[string]*SentinelMaster)

	var voted int
	for alive := len(sentinels); ; alive-- {
		if alive == 0 {
			switch {
			case cntx.Err() != context.DeadlineExceeded && cntx.Err() != nil:
				s.printf("sentinel masters canceled (%v)", cntx.Err())
				return nil, errors.Trace(cntx.Err())
			case voted != len(sentinels):
				s.printf("sentinel masters voted = (%d/%d) masters = %d (%v)", voted, len(sentinels), len(masters), cntx.Err())
			}
			if voted < majority {
				return nil, errors.Errorf("lost majority (%d/%d)", voted, len(sentinels))
			}
			return masters, nil
		}
		select {
		case <-cntx.Done():
			switch {
			case cntx.Err() != context.DeadlineExceeded:
				s.printf("sentinel masters canceled (%v)", cntx.Err())
				return nil, errors.Trace(cntx.Err())
			default:
				s.printf("sentinel masters voted = (%d/%d) masters = %d (%v)", voted, len(sentinels), len(masters), cntx.Err())
			}
			if voted < majority {
				return nil, errors.Errorf("lost majority (%d/%d)", voted, len(sentinels))
			}
			return masters, nil
		case m := <-results:
			if m == nil {
				continue
			}
			for groupName, master := range m {
				if current[groupName] == nil || current[groupName].Epoch < master.Epoch {
					current[groupName] = master
					masters[groupName] = master.Addr
				}
			}
			voted += 1
		}
	}
}

type MonitorConfig struct {
	Quorum          int
	ParallelSyncs   int
	DownAfter       time.Duration
	FailoverTimeout time.Duration

	NotificationScript   string
	ClientReconfigScript string
}

func (s *Sentinel) monitorGroupsCommand(client *Client, sentniel string, config *MonitorConfig, groups map[string]*net.TCPAddr) error {
	defer func() {
		if !client.isRecyclable() {
			client.Close()
		}
	}()
	var groupNames []string
	for groupName := range groups {
		groupNames = append(groupNames, s.NodeName(groupName))
	}
	if err := s.removeCommand(client, groupNames); err != nil {
		return err
	}
	go func() {
		for groupName, tcpAddr := range groups {
			var ip, port = tcpAddr.IP.String(), tcpAddr.Port
			client.Send("SENTINEL", "monitor", s.NodeName(groupName), ip, port, config.Quorum)
		}
		if len(groups) != 0 {
			client.Flush()
		}
	}()
	for range groups {
		_, err := client.Receive()
		if err != nil {
			return errors.Trace(err)
		}
	}
	go func() {
		for groupName := range groups {
			var args = []interface{}{"set", s.NodeName(groupName)}
			if config.ParallelSyncs != 0 {
				args = append(args, "parallel-syncs", config.ParallelSyncs)
			}
			if config.DownAfter != 0 {
				args = append(args, "down-after-milliseconds", int(config.DownAfter/time.Millisecond))
			}
			if config.FailoverTimeout != 0 {
				args = append(args, "failover-timeout", int(config.FailoverTimeout/time.Millisecond))
			}
			if s.Auth != "" {
				args = append(args, "auth-pass", s.Auth)
			}
			if config.NotificationScript != "" {
				args = append(args, "notification-script", config.NotificationScript)
			}
			if config.ClientReconfigScript != "" {
				args = append(args, "client-reconfig-script", config.ClientReconfigScript)
			}
			client.Send("SENTINEL", args...)
		}
		if len(groups) != 0 {
			client.Flush()
		}
	}()
	for range groups {
		_, err := client.Receive()
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (s *Sentinel) monitorGroupsDispatch(ctx context.Context, sentinel string, timeout time.Duration, config *MonitorConfig, groups map[string]*net.TCPAddr) error {
	var err = s.dispatch(ctx, sentinel, timeout, func(c *Client) error {
		return s.monitorGroupsCommand(c, sentinel, config, groups)
	})
	if err != nil {
		switch errors.Cause(err) {
		case context.Canceled:
			return nil
		default:
			return err
		}
	}
	return nil
}

func (s *Sentinel) MonitorGroups(sentinels []string, timeout time.Duration, config *MonitorConfig, groups map[string]string) error {
	cntx, cancel := context.WithTimeout(s.Context, timeout)
	defer cancel()

	resolve := make(map[string]*net.TCPAddr)

	var exit = make(chan error, 1)

	go func() (err error) {
		defer func() {
			exit <- err
		}()
		for groupName, addr := range groups {
			if err := cntx.Err(); err != nil {
				return errors.Trace(err)
			}
			tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
			if err != nil {
				s.printf("sentinel monitor resolve tcp address of %s failed, %s", addr, err)
				return errors.Trace(err)
			}
			resolve[groupName] = tcpAddr
		}
		return nil
	}()

	select {
	case <-cntx.Done():
		if cntx.Err() != context.DeadlineExceeded {
			s.printf("sentinel monitor canceled (%v)", cntx.Err())
		} else {
			s.printf("sentinel montior resolve tcp address (%v)", cntx.Err())
		}
		return errors.Trace(cntx.Err())
	case err := <-exit:
		if err != nil {
			return err
		}
	}

	timeout += time.Second * 5
	results := make(chan error, len(sentinels))

	for i := range sentinels {
		go func(sentinel string) {
			err := s.monitorGroupsDispatch(cntx, sentinel, timeout, config, resolve)
			if err != nil {
				s.errorf("sentinel-[%s] monitor failed. err:%s", sentinel, err.Error())
			}
			results <- err
		}(sentinels[i])
	}

	var last error
	for range sentinels {
		select {
		case <-cntx.Done():
			if last != nil {
				return last
			}
			return errors.Trace(cntx.Err())
		case err := <-results:
			if err != nil {
				last = err
			}
		}
	}
	return last
}

func (s *Sentinel) removeCommand(client *Client, names []string) error {
	defer func() {
		if !client.isRecyclable() {
			client.Close()
		}
	}()
	exists, err := s.existsCommand(client, names)
	if err != nil {
		return err
	}
	go func() {
		var pending int
		for _, name := range names {
			if !exists[name] {
				continue
			}
			pending++
			client.Send("SENTINEL", "remove", name)
		}
		if pending != 0 {
			client.Flush()
		}
	}()
	for _, name := range names {
		if !exists[name] {
			continue
		}
		_, err := client.Receive()
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (s *Sentinel) removeGroupsDispatch(ctx context.Context, sentinel string, timeout time.Duration, groups map[string]bool) error {
	var groupNames []string
	for groupName := range groups {
		groupNames = append(groupNames, s.NodeName(groupName))
	}
	var err = s.dispatch(ctx, sentinel, timeout, func(c *Client) error {
		return s.removeCommand(c, groupNames)
	})
	if err != nil {
		switch errors.Cause(err) {
		case context.Canceled:
			return nil
		default:
			return err
		}
	}
	return nil
}

func (s *Sentinel) RemoveGroups(sentinels []string, timeout time.Duration, groups map[string]bool) error {
	cntx, cancel := context.WithTimeout(s.Context, timeout)
	defer cancel()

	timeout += time.Second * 5
	results := make(chan error, len(sentinels))

	for i := range sentinels {
		go func(sentinel string) {
			err := s.removeGroupsDispatch(cntx, sentinel, timeout, groups)
			if err != nil {
				s.errorf("sentinel-[%s] remove failed. err:%s", sentinel, err.Error())
			}
			results <- err
		}(sentinels[i])
	}

	var last error
	for range sentinels {
		select {
		case <-cntx.Done():
			if last != nil {
				return last
			}
			return errors.Trace(cntx.Err())
		case err := <-results:
			if err != nil {
				last = err
			}
		}
	}
	return last
}

func (s *Sentinel) removeGroupsAllDispatch(ctx context.Context, sentinel string, timeout time.Duration) error {
	var err = s.dispatch(ctx, sentinel, timeout, func(c *Client) error {
		masters, err := s.mastersCommand(c)
		if err != nil {
			return err
		}
		var groupNames []string
		for groupName := range masters {
			groupNames = append(groupNames, s.NodeName(groupName))
		}
		return s.removeCommand(c, groupNames)
	})
	if err != nil {
		switch errors.Cause(err) {
		case context.Canceled:
			return nil
		default:
			return err
		}
	}
	return nil
}

func (s *Sentinel) RemoveGroupsAll(sentinels []string, timeout time.Duration) error {
	cntx, cancel := context.WithTimeout(s.Context, timeout)
	defer cancel()

	timeout += time.Second * 5
	results := make(chan error, len(sentinels))

	for i := range sentinels {
		go func(sentinel string) {
			err := s.removeGroupsAllDispatch(cntx, sentinel, timeout)
			if err != nil {
				s.errorf("sentinel-[%s] remove failed. err:%s", sentinel, err.Error())
			}
			results <- err
		}(sentinels[i])
	}

	var last error
	for range sentinels {
		select {
		case <-cntx.Done():
			if last != nil {
				return last
			}
			return errors.Trace(cntx.Err())
		case err := <-results:
			if err != nil {
				last = err
			}
		}
	}
	return last
}

type SentinelGroup struct {
	Master map[string]string   `json:"master"`
	Slaves []map[string]string `json:"slaves,omitempty"`
}

func (s *Sentinel) MastersAndSlavesClient(client *Client) (map[string]*SentinelGroup, error) {
	defer func() {
		if !client.isRecyclable() {
			client.Close()
		}
	}()
	masters, err := s.mastersCommand(client)
	if err != nil {
		return nil, err
	}
	var groupNames []string
	for groupName := range masters {
		groupNames = append(groupNames, s.NodeName(groupName))
	}
	slaves, err := s.slavesCommand(client, groupNames)
	if err != nil {
		return nil, err
	}
	results := make(map[string]*SentinelGroup, len(masters))
	for groupName, master := range masters {
		var name = s.NodeName(groupName)
		results[name] = &SentinelGroup{
			Master: master, Slaves: slaves[name],
		}
	}
	return results, nil
}

func (s *Sentinel) MastersAndSlaves(sentinel string, timeout time.Duration) (map[string]*SentinelGroup, error) {
	var results map[string]*SentinelGroup
	var err = s.do(sentinel, timeout, func(c *Client) error {
		m, err := s.MastersAndSlavesClient(c)
		if err != nil {
			return err
		}
		results = m
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Sentinel) FlushConfig(sentinel string, timeout time.Duration) error {
	return s.do(sentinel, timeout, func(c *Client) error {
		_, err := c.Do("SENTINEL", "flushconfig")
		if err != nil {
			return err
		}
		return nil
	})
}

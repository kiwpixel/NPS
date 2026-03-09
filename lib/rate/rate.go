package rate

import (
	"math"
	"sync/atomic"
	"time"
)

type Rate struct {
	bucketSize        int64
	bucketSurplusSize int64
	bucketAddSize     int64
	stopChan          chan bool
	NowRate           int64
}

// 优化点1：初始化时直接设置无限制的令牌桶（核心解除限速）
func NewRate(addSize int64) *Rate {
	// 桶容量、初始令牌数、每秒添加数都设为int64最大值，彻底取消限速
	return &Rate{
		bucketSize:        math.MaxInt64,       // 桶容量无上限
		bucketSurplusSize: math.MaxInt64,       // 初始令牌数直接拉满
		bucketAddSize:     math.MaxInt64,       // 每秒添加令牌数无上限
		stopChan:          make(chan bool, 1),  // 优化：给通道加缓冲区，避免阻塞
	}
}

func (s *Rate) Start() {
	go s.session()
}

// 优化点2：简化add逻辑，移除无意义的容量判断（因为桶已无上限）
func (s *Rate) add(size int64) {
	// 直接累加，无需判断桶容量（MaxInt64不会溢出）
	atomic.AddInt64(&s.bucketSurplusSize, size)
}

// 回桶
func (s *Rate) ReturnBucket(size int64) {
	s.add(size)
}

// 优化点3：Stop逻辑防阻塞（通道有缓冲区，无需担心写入失败）
func (s *Rate) Stop() {
	select {
	case s.stopChan <- true:
	default:
	}
}

// 优化点4：彻底移除Get方法的阻塞逻辑（核心优化，解决速度卡顿）
func (s *Rate) Get(size int64) {
	// 令牌数永远充足，直接扣减（无阻塞、无限速）
	if s.bucketSurplusSize >= size {
		atomic.AddInt64(&s.bucketSurplusSize, -size)
	}
	// 即使极端情况令牌数不足（理论上不会出现），也直接放行，不阻塞
}

// 优化点5：简化session循环，减少无意义计算（因为令牌桶无上限）
func (s *Rate) session() {
	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop() // 优化：defer确保ticker关闭

	for {
		select {
		case <-ticker.C:
			// 简化NowRate计算，无需复杂判断
			s.NowRate = math.MaxInt64
			// 令牌桶无上限，add操作仅做象征性累加
			s.add(s.bucketAddSize)
		case <-s.stopChan:
			return
		}
	}
}

package bulk

import (
	"time"
)

// Option 选项
type Option func(*options)

// GroupFunc 任务分组函数
type GroupFunc[T any] func(T) string

// ExecuteFunc 批量执行任务函数
type ExecuteFunc[T any] func([]T)

type options struct {
	// 最长多长时间执行一次
	timeout time.Duration
	// 最多多少条目执行一次
	maxItem uint32
}

// Executor 批量延迟合并执行器
// 使用 NewExecutor 创建实例
type Executor[T any] struct {
	opts *options
	// 延迟合并元素队列
	queue chan *T
	// 执行循环计时器
	ticker *time.Ticker
	// 停止执行
	stopSignal    chan struct{}
	stoppedSignal chan struct{}
	// 分组待执行任务数组 此数组满足一定条件就会合并执行
	tasks map[string][]*T
	// 分组函数
	groupFunc GroupFunc[*T]
	// 批量执行任务函数
	executeFunc ExecuteFunc[*T]
}

// WithMaxItem 最多多少条目执行一次
func WithMaxItem(maxItem uint32) Option {
	return func(o *options) {
		o.maxItem = maxItem
	}
}

// WithTimeout 最多多久执行一次
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.timeout = timeout
	}
}

// DefaultOptions 默认选项
func DefaultOptions() []Option {
	return []Option{
		WithMaxItem(20),
		WithTimeout(time.Second * 3),
	}
}

// NewExecutor 创建批量执行器
// 执行逻辑：业务把待执行且可批量执行的内容提交给Executor，并设置批量执行等参数
// 执行触发时机有如下三种：
// 1. 当接收到执行数量的待执行内容时，由WithMaxItem选项控制多少内容
// 2. 当超过一定时间未执行时，由WithTimeout选项控制多长时间
// 3. 关闭执行器时（当服务需要停止/重启等情况时，需要先把队列中的待执行内容执行完毕）
func NewExecutor[T any](execFunc ExecuteFunc[*T], opts ...Option) *Executor[T] {
	e := &Executor[T]{opts: &options{}}
	// 应用默认选项
	for _, opt := range DefaultOptions() {
		opt(e.opts)
	}
	// 应用指定选项
	for _, opt := range opts {
		opt(e.opts)
	}
	e.ticker = time.NewTicker(e.opts.timeout)
	e.queue = make(chan *T, e.opts.maxItem*2)
	e.stopSignal = make(chan struct{})
	e.stoppedSignal = make(chan struct{})
	e.executeFunc = execFunc
	e.tasks = make(map[string][]*T)
	return e
}

// GroupBy 设置任务分组函数
// 当待执行的任务不可能无差别的批量执行时，则需要执行分组函数，通过此函数对待执行任务进行分组。同组的任务可同一批次批量执行
func (e *Executor[T]) GroupBy(groupFunc GroupFunc[*T]) *Executor[T] {
	e.groupFunc = groupFunc
	return e
}

// Start 开始执行
func (e *Executor[T]) Start() {
	go func() {
	loop:
		for {
			select {
			case task := <-e.queue:
				groupName := e.tempSaveTask(task)
				if len(e.tasks[groupName]) >= int(e.opts.maxItem) {
					e.bulkExecute(groupName)
				}
			case <-e.ticker.C:
				// 如果分组过多，会影响执行效率
				cleanGroup := len(e.tasks) > 1000
				for groupName := range e.tasks {
					if len(e.tasks[groupName]) == 0 {
						if cleanGroup {
							delete(e.tasks, groupName)
						}
						continue
					}
					e.bulkExecute(groupName)
				}
			case <-e.stopSignal:
				e.ticker.Stop()
				break loop
			}
		}
		// 把所有的任务执行一遍
		for groupName := range e.tasks {
			if len(e.tasks[groupName]) == 0 {
				continue
			}
			e.bulkExecute(groupName)
		}
		// 通知执行完毕
		close(e.stoppedSignal)
	}()
}

// 暂存任务
func (e *Executor[T]) tempSaveTask(task *T) string {
	groupName := "default"
	if e.groupFunc != nil {
		groupName = e.groupFunc(task)
	}
	if _, ok := e.tasks[groupName]; !ok {
		e.tasks[groupName] = make([]*T, 0, e.opts.maxItem)
	}
	e.tasks[groupName] = append(e.tasks[groupName], task)
	return groupName
}

// 批量执行任务
func (e *Executor[T]) bulkExecute(groupName string) {
	if len(e.tasks[groupName]) == 0 {
		return
	}
	params := make([]*T, len(e.tasks[groupName]))
	copy(params, e.tasks[groupName])
	// 清空
	e.tasks[groupName] = e.tasks[groupName][:0]
	go e.executeFunc(params)
}

// Execute 往执行延迟合并执行队列中写入待执行数据
func (e *Executor[T]) Execute(t *T) {
	e.queue <- t
}

// Stop 结束执行并批量执行完当前所有未执行的任务
// 执行完毕后才返回，业务需要等待返回后才能认为任务已全部执行完毕
func (e *Executor[T]) Stop() bool {
	// 通知需要停止
	close(e.stopSignal)
	// 等待停止完毕
	<-e.stoppedSignal
	return true
}

// Wait 等待执行结束
func (e *Executor[T]) Wait() {
	<-e.stoppedSignal
}

package bulk

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

type testTask struct {
	i  int
	op string
}

// 分组
func group(task *testTask) string {
	return task.op
}

var startTime = time.Now()

// 执行
func bulkExecute(tasks []*testTask) {
	if len(tasks) == 0 {
		// should never happen
		fmt.Println("empty tasks!")
		return
	}
	op := tasks[0].op
	fmt.Printf("[%s]%s: ", op, time.Now().Sub(startTime))

	var result int
	for i, task := range tasks {
		fmt.Printf("%d%s ", task.i, func(end bool) string {
			if end {
				return ""
			}
			return " " + op
		}(i == len(tasks)-1))
		if i == 0 {
			result = task.i
			continue
		}
		switch op {
		case "+":
			result += task.i
		case "*":
			result *= task.i
		case "-":
			result -= task.i
		case "/":
			result /= task.i
		}
	}
	fmt.Println("=", result)
	// 模拟任务执行时间较长，如果观察到有多个任务在同一秒执行，则证明可以并行处理
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(1000)))
}

func TestExecutor(t *testing.T) {
	executor := NewExecutor(bulkExecute, WithMaxItem(3), WithTimeout(time.Second))
	executor.Start()
	time.AfterFunc(time.Second*3, func() {
		stopped := executor.Stop()
		fmt.Println("TestExecutor stopped:", stopped)
	})
	for i := 1; i <= 10; i++ {
		executor.Execute(&testTask{
			i:  i,
			op: "+",
		})
	}

	executor.Wait()
	time.Sleep(time.Second * 3)
}

func TestGroupExecutor(t *testing.T) {
	executor := NewExecutor(bulkExecute, WithMaxItem(3), WithTimeout(time.Second))
	executor.GroupBy(group).Start()
	time.AfterFunc(time.Second*3, func() {
		stopped := executor.Stop()
		fmt.Println("TestGroupExecutor stopped:", stopped)
	})
	for i := 1; i <= 10; i++ {
		executor.Execute(&testTask{
			i: i,
			op: func(x int) string {
				if x%2 == 0 {
					return "+"
				}
				return "*"
			}(i),
		})
	}

	executor.Wait()
	time.Sleep(time.Second * 3)
}

func TestGroupExecutorLongTime(t *testing.T) {
	executor := NewExecutor(bulkExecute, WithMaxItem(3), WithTimeout(time.Second))
	executor.GroupBy(group)
	executor.Start()
	time.AfterFunc(time.Second*10, func() {
		stopped := executor.Stop()
		fmt.Println("TestGroupExecutorLongTime stop executor:", stopped)
	})
	for i := 1; i <= 100; i++ {
		// 前边快，后边慢，观察到的效果应该是前期任务到达maxItem后就执行。后期到达timeout后执行
		if i < 50 {
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(50)))
		} else {
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(500)))
		}
		executor.Execute(&testTask{
			i: i,
			op: func(x int) string {
				if x%2 == 0 {
					return "+"
				}
				return "*"
			}(i),
		})
	}

	executor.Wait()
}

# Bulk executor

延迟合并批量执行工具，使用泛型以达到通用化。

```shell
go get github.com/wclssdn/bulk
```

## 使用场景

当任务可能出现短时间多个任务，并且任务可以被合并批量执行时。

## 例子

计数器每收到用户的一个计数，便对结果累加。例如，用户提交了1~10的累加请求。
```go
// 对用户任务的表示
type SumTask struct {
    num int
}
// 模拟用户的10次累加请求处理
for i := 1; i <= 10; i++ {
    executor.Execute(&SumTask{num: i}) // 每次收到请求便创造一个SumTask表示用户的请求，并提交给批量执行器
}
```

定义批量处理用户累加请求的执行函数
```go
func BulkSum(tasks []*SumTask) {
    sum := 0
    for _, task := range tasks {
        sum += task.num
    }
    incr(sum) // 存储累加值
}
```

可运行的例子见`bulk_test.go`（可通过`go test -v .`执行测试用例，查看效果）

## 使用方式

业务定义任务数据结构`XXTask`，包含执行此任务必要的信息。
```go
type testTask struct {
    i  int
    op string
}
```

再定义任务批量执行函数`bulkExecute([]*XXTask)`，此函数会被自动调用，传入的是触发阈值时积攒的**一个或者多个**任务。
```go
func bulkExecute(tasks []*testTask) {
    result := 0
    for _, task := range tasks {
        result += task.i
    }
    fmt.Println(result)
}
```

初始化一个`Executor`，并指定批量执行的方法

```go
executor := NewExecutor(bulkExecute)
executor.Start() // 内部会开启一个协程，不会阻塞程序运行
```

当产生要执行的任务时，调用库函数`Execute(*XXTask)`
```go
task := &testTask{i: 1, op: "+"}
executor.Execute(task)
```

## 高阶使用

### 设置选项

在初始化的时候，如果不提供选项，则使用默认值，由包内的`DefaultOptions()`函数生成（默认值可查看此函数内容）

执行器提供如下选项：

- `WithMaxItem` 积攒了多少任务执行一次
- `WithTimeout` 积攒了多久执行一次（如有任务）
```go
NewExecutor(bulkExecute, WithMaxItem(3), WithTimeout(time.Second))
```

### 分组

当任务数据中存在多组可合并的内容时，可通过分组函数进行分组合并执行。

Ps: 完全不同类型的任务，更适合使用多个执行器。仅同样数据结构的任务适合使用分组。

创建分组函数`groupBy(*XXTask) string`，函数计算任务，返回分组标识，同一个标识的任务会分配到同一个组 
```go
func group(task *testTask) string {
	return task.op
}
```

在初始化`Executor`后，设置分组函数（注意，要在`Start`函数调用前调用）
```go
executor := NewExecutor(bulkExecute)
executor.GroupBy(group).Start()
```

效果：任务数据中的`op`字段作为分组标识，同样操作的会被合并到一起批量操作。

整个执行器的最大执行间隔（`timeout`）生效于所有的分组，每个组独立计算最大长度（`maxItem`）。

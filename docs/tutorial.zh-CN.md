# Bloom Filter 项目原理与实现教程

## 从一个问题开始

有人问过一个很典型的问题：

> Google 是如何在毫秒级别的时间内判断注册的用户是否重复？

这个问题背后不是“某一个神奇函数”，而是一类常见的工程设计：**不要让每一次查询都直接打到权威数据库**。

假设一个系统有几十亿个已注册用户名。新用户输入用户名时，服务端需要判断：

```text
这个用户名是否已经被占用？
```

最直接的方法是查数据库：

```sql
select id from users where username = 'alice';
```

这当然正确，但在超大规模系统里，如果每一次输入、每一次试探、每一次机器人请求都查数据库，数据库会承受巨大压力。尤其是很多用户名其实根本不存在，数据库查完只是为了回答“不存在”。

Bloom Filter 的价值就在这里：它可以用很小的内存，先在服务层快速回答一类问题：

```text
如果 Bloom Filter 说“不存在”：这个用户名在过滤器里一定不存在。
如果 Bloom Filter 说“可能存在”：它只表示可能存在，业务系统需要再查权威存储做最终确认。
```

所以这个项目本身不是数据库，也不包含查数据库的功能。它只提供“可能存在/一定不存在”的快速判断能力；数据库、唯一索引、注册写入等都属于调用方业务系统。

它不是替代数据库，而是一个**负缓存**：

```text
大量明显不存在的查询 -> Bloom Filter 直接拦下
少量可能存在的查询 -> 业务系统回查数据库或其他权威存储确认
```

所以更准确的流程是：

```text
用户输入 username
        |
        v
Bloom Filter 查询
        |
        +-- 不存在 -> 在过滤器数据及时同步的前提下，可以认为没注册过
        |
        +-- 可能存在 -> 调用方查数据库确认，避免误判
```

这类设计常见于用户名/邮箱查重、爬虫 URL 去重、缓存穿透防护、黑名单判断、广告/推荐去重等场景。

需要注意：这里用 Google 作为问题来源，不代表 Google 的真实注册系统只靠 Bloom Filter。真实大厂系统通常会叠加缓存、分片索引、数据库唯一约束、风控、异步校验等多层机制。Bloom Filter 解决的是其中一个非常典型的问题：**如何用很少的内存快速排除“一定不存在”的请求**。

## Bloom Filter 解决了什么

Bloom Filter 适合回答集合成员判断问题：

```text
key 是否在集合 S 里？
```

它的回答有两个特点：

```text
回答 false：一定不存在
回答 true：可能存在
```

也就是说，Bloom Filter 允许**假阳性**，但不允许**假阴性**。

```text
假阳性：明明没加入过，却回答“可能存在”
假阴性：明明加入过，却回答“不存在”
```

Bloom Filter 的核心保证是：

```text
不会有假阴性；
可能有假阳性；
假阳性概率可以通过参数控制。
```

这就是为什么它适合做“负缓存”。当它回答“不存在”时，可以相信；当它回答“可能存在”时，库本身不会继续查库，需要调用方按业务需要回查权威存储。

## 这个项目提供了什么

这个项目是一个 Go 语言 Bloom Filter 库，模块名是：

```go
github.com/huxint/bloomfilter
```

它提供四类核心能力：

```text
经典 Bloom Filter：Add + MightContain，不支持删除
Counting Bloom Filter：Add + MightContain + Remove，支持删除
二进制序列化：Save / Load
mmap 文件映射：CreateMmap / OpenMmap，用于超大过滤器
```

它不提供这些能力：

```text
不连接数据库
不执行 SQL
不保证用户名最终唯一
不替代数据库唯一索引
```

典型用法：

```go
f, err := bloomfilter.New(1_000_000, 0.001)
if err != nil {
    panic(err)
}

f.AddString("alice")

if !f.MightContainString("bob") {
    // bob 一定不存在
}
```

这里：

```text
n = 1_000_000  表示预计放入 100 万个元素
p = 0.001      表示目标假阳性率约为 0.1%
```

## 项目结构

核心文件如下：

```text
filter.go                 公共接口、共享 core、参数校验、m/k 计算
bloom.go                  经典 Bloom Filter 实现
counting.go               Counting Bloom Filter 实现
codec.go                  二进制 header、Save、Load、ReadFrom、WriteTo
mmap.go                   mmap 文件过滤器创建和打开
doc.go                    包文档
internal/hashing/         默认哈希和 double hashing 索引生成
internal/storage/         内存和 mmap 存储抽象
examples/                 使用示例
*_test.go                 测试和 benchmark
```

根目录放公共包 `bloomfilter`。这是 Go 单包库常见布局，用户导入路径保持简洁：

```go
import "github.com/huxint/bloomfilter"
```

`internal/` 下面是内部实现细节，外部项目不能直接导入。

## 核心数据结构

这个项目的共享核心结构在 `filter.go`：

```go
type core struct {
    m, k, n  uint64
    hasher   hashing.Hasher
    hashID   uint8
    kind     Kind
    cellBits uint8
    store    storage.Region
}
```

这些字段是理解整个项目的关键：

```text
m        一共有多少个 cell。经典 Bloom 中就是多少个 bit
k        每个 key 要映射到多少个位置
n        Add 次数，Counting 删除时会减少
hasher   哈希器
hashID   持久化时记录哈希算法 ID
kind     过滤器类型：Bloom 或 Counting
cellBits 每个 cell 占几 bit。Bloom 是 1，Counting 是 4
store    底层存储，可以是内存，也可以是 mmap 文件
```

经典 Bloom Filter 的真实存储不是 `[]bool`，而是 `[]byte`：

```text
1 byte = 8 bit
```

设置第 `idx` 个 bit 时：

```go
b[idx>>3] |= 1 << (idx & 7)
```

这里：

```text
idx >> 3  等价于 idx / 8，找到第几个 byte
idx & 7   等价于 idx % 8，找到这个 byte 里的第几个 bit
```

用 bit 而不是 bool，可以把内存压到非常低。

## n、p、m、k 分别是什么

用户创建过滤器时传入：

```go
New(n, p)
```

其中：

```text
n  预计插入多少元素
p  目标假阳性率
```

项目根据 `n` 和 `p` 自动计算：

```text
m  需要多少个 bit/cell
k  每个 key 设置/检查多少个位置
```

计算逻辑在 `filter.go`：

```go
m = -n * ln(p) / (ln2)^2
k = m / n * ln2
```

直观理解：

```text
p 越小，表示希望误判率越低
需要的 m 越大，也就是 bit 数组越大
k 通常也会变大，也就是每个 key 检查更多位置
```

常见规模大概是：

```text
p = 0.01    -> k 约为 7
p = 0.001   -> k 约为 10
p = 0.0001  -> k 约为 13
```

`k` 不是越大越好。固定 `m` 和 `n` 时，`k` 太小区分度不够；`k` 太大会把太多 bit 置 1，反而让假阳性变高。本项目使用接近理论最优的 `k`。

## Add 的过程

假设执行：

```go
f.AddString("alice")
```

核心逻辑在 `bloom.go`：

```go
func (f *BloomFilter) Add(key []byte) {
    h1, h2 := f.hasher.Hash128(key)
    b := f.store.Bytes()
    for i := uint64(0); i < f.k; i++ {
        idx := hashing.Index(h1, h2, i, f.m)
        b[idx>>3] |= 1 << (idx & 7)
    }
    f.n++
}
```

步骤如下：

```text
1. 对 key 计算哈希，得到 h1、h2
2. 用 h1、h2 推导出 k 个位置
3. 把这 k 个位置对应的 bit 设置为 1
4. n 加 1
```

假设：

```text
m = 100
k = 5
h1 = 7
h2 = 13
```

生成的位置是：

```text
i = 0 -> (7 + 0*13) % 100 = 7
i = 1 -> (7 + 1*13) % 100 = 20
i = 2 -> (7 + 2*13) % 100 = 33
i = 3 -> (7 + 3*13) % 100 = 46
i = 4 -> (7 + 4*13) % 100 = 59
```

所以 `"alice"` 会设置这些位置：

```text
7, 20, 33, 46, 59
```

Bloom Filter 不保存 `"alice"` 本身，只保存它影响过哪些 bit。

## MightContain 的过程

查询：

```go
f.MightContainString("alice")
```

核心逻辑在 `bloom.go`：

```go
func (f *BloomFilter) MightContain(key []byte) bool {
    h1, h2 := f.hasher.Hash128(key)
    b := f.store.Bytes()
    for i := uint64(0); i < f.k; i++ {
        idx := hashing.Index(h1, h2, i, f.m)
        if b[idx>>3]&(1<<(idx&7)) == 0 {
            return false
        }
    }
    return true
}
```

查询时会重新计算相同的 `h1`、`h2`，再得到相同的 `k` 个位置。

判断规则是：

```text
只要有一个位置是 0 -> 一定不存在
所有位置都是 1     -> 可能存在
```

为什么“所有位置都是 1”不能说“一定存在”？

因为这些 bit 可能是别的 key 设置出来的。

例如 `"alice"` 对应的位置是：

```text
7, 20, 33, 46, 59
```

但系统从来没有添加过 `"alice"`。只是其他 key 刚好设置了这些位置：

```text
"tom"  设置了 7, 20
"jack" 设置了 33
"bob"  设置了 46, 59
```

这时查询 `"alice"` 会发现所有位置都是 1，于是回答：

```text
可能存在
```

这就是假阳性。

## 为什么不是 k 个真正独立哈希

理论上，Bloom Filter 可以理解为使用 `k` 个哈希函数：

```text
hash_1(key)
hash_2(key)
hash_3(key)
...
hash_k(key)
```

但工程上真的计算 `k` 个独立哈希函数会比较慢。

这个项目采用常见的 **double hashing**：

```text
只计算两个基础哈希值 h1、h2
再推导出 k 个位置
```

索引公式在 `internal/hashing/hashing.go`：

```go
func Index(h1, h2, i, m uint64) uint64 {
    return (h1 + i*h2) % m
}
```

也就是：

```text
idx_0 = (h1 + 0*h2) % m
idx_1 = (h1 + 1*h2) % m
idx_2 = (h1 + 2*h2) % m
...
idx_{k-1} = (h1 + (k-1)*h2) % m
```

所以准确说：

```text
不是实际计算 k 个哈希；
而是计算 h1/h2 两个基础哈希；
再生成并检查 k 个位置。
```

如果两个 key 的 `h1`、`h2` 完全相同，那么它们生成的 `k` 个位置也完全相同。这个概率极低，因为这里等价于 128 位哈希结果相同。

但即使 `h1`、`h2` 不同，不同 key 也可能在取模后碰到相同 bit。这很正常，也是 Bloom Filter 假阳性的来源。

## k 个位置会不会重复

会，理论上可能。

公式是：

```text
idx_i = (h1 + i*h2) % m
```

如果 `h2` 和 `m` 的关系导致短周期，那么同一个 key 的前 `k` 个位置里可能出现重复。

极端例子：

```text
m = 10
h1 = 1
h2 = 5
k = 6
```

生成位置：

```text
1, 6, 1, 6, 1, 6
```

虽然循环了 6 次，但实际只有两个不同位置。

这会让这个 key 的“有效 k”变小，理论上会略微增加假阳性。但在正常参数下：

```text
m 通常很大
k 通常较小
h2 来自混合后的 64 位哈希
```

所以前 `k` 个位置里重复的概率通常很低。本项目没有额外强制去重，因为去重需要额外开销，而且收益有限。

## 哈希实现

默认哈希器是 `FNV128a`：

```go
type FNV128a struct{}
```

它会对 key 计算 FNV-1a-128，然后拆成两个 64 位数：

```text
h1, h2
```

再分别经过 `splitmix64` finalizer 做混合。这样做是为了改善 FNV 低位扩散较弱的问题，让 double hashing 生成的位置更均匀。

默认哈希有两个重要要求：

```text
确定性：同一个 key 在不同进程、不同时间必须得到相同结果
稳定性：持久化文件重新加载后，查询结果必须不变
```

所以不能随便换哈希算法。这个项目的文件 header 里记录了 `hashID`，默认哈希的 ID 是 0。只要默认哈希语义变了，旧文件就可能无法正确查询。

目前项目不提供公开的自定义哈希接口。内部虽然有 `Hasher` 接口，但它在 `internal/hashing` 包中，外部不能导入。这是有意保持 API 简洁，也避免持久化兼容性问题。

## Counting Bloom Filter

经典 Bloom Filter 不能删除。

原因是一个 bit 可能被多个 key 共用：

```text
alice 设置了 bit 20
bob   也设置了 bit 20
```

如果删除 alice 时直接把 bit 20 清零，就可能误伤 bob，导致 bob 查询时出现假阴性。

Counting Bloom Filter 的做法是：

```text
每个位置不再存 1 bit
而是存一个计数器
```

添加时：

```text
counter += 1
```

删除时：

```text
counter -= 1
```

查询时：

```text
所有 counter 都大于 0 -> 可能存在
只要有一个 counter 是 0 -> 一定不存在
```

本项目里的 Counting Filter 每个计数器使用 4 bit：

```text
4 bit 可以表示 0 到 15
```

所以它的内存大约是经典 Bloom Filter 的 4 倍：

```text
经典 Bloom：每个 cell 1 bit
Counting：  每个 cell 4 bit
```

为什么最大是 15？

因为 4 bit 的最大值就是：

```text
1111（二进制）= 15
```

这是内存和功能之间的折中。对亿级、十亿级数据来说，如果每个计数器用 `uint8`，内存会变成经典 Bloom 的 8 倍；如果用 `uint16`，会变成 16 倍。

本项目选择 4 bit，是为了在支持删除的同时尽量控制内存。

## Counting 的饱和问题

Counting Filter 的计数器最大只能到 15，因此会饱和。

代码逻辑是：

```text
Add 时，如果 counter < 15，就加 1；如果已经是 15，就保持 15
Remove 时，如果 counter == 15，就不减；因为它可能已经饱和，无法知道真实值
```

这意味着：

```text
饱和位置不能被安全地精确删除
```

这不是 bug，而是 4-bit 计数器的已知限制。

使用 Counting Filter 时应遵守：

```text
只 Remove 确实 Add 过的元素
如果删除需求很重，且重复/碰撞很高，要考虑更大的过滤器或更大的计数器设计
```

同一个 key 的 `k` 个位置如果出现重复，会不会破坏删除？

一般不会，因为 Add 和 Remove 是对称的：

```text
Add    对重复位置加多次
Remove 对重复位置也减多次
```

真正需要担心的是重复位置会让计数器更容易饱和。

## 存储抽象

这个项目没有把过滤器绑定死在普通内存上，而是抽象了一个 `Region`：

```go
type Region interface {
    Bytes() []byte
    Header() []byte
    ReadOnly() bool
    Sync() error
    Close() error
}
```

这样上层算法只关心：

```go
b := f.store.Bytes()
```

至于这些 bytes 来自哪里，上层不需要知道。

当前有两类存储：

```text
内存存储：storage.NewMem / storage.WrapMem
mmap 存储：storage.MapFile
```

这让同一套 `Add` / `MightContain` 逻辑既可以操作内存数组，也可以操作映射到文件的内存区域。

## mmap 适合什么场景

普通内存过滤器适合中小规模数据。比如几百万、几千万 key，直接放内存很方便。

但如果过滤器很大，例如十亿、百亿级 key，对应的 bit 数组可能是 GB 级别。此时可以使用 mmap：

```go
mf, err := bloomfilter.CreateMmap("big.blmf", bloomfilter.KindBloom, 10_000_000_000, 0.001)
if err != nil {
    panic(err)
}
defer mf.Close()

mf.AddString("alice")
_ = mf.Sync()
```

mmap 的价值是：

```text
过滤器直接映射到文件
重启后可以 OpenMmap 重新打开
不需要从头重建
操作接口和普通 Filter 基本一致
```

只读服务可以这样打开：

```go
ro, err := bloomfilter.OpenMmap("big.blmf", true)
if err != nil {
    panic(err)
}
defer ro.Close()

if ro.MightContain([]byte("alice")) {
    // 可能存在
}
```

只读 mmap 上调用 `Add` 会 panic，这是为了避免错误修改只读映射。

## 序列化格式

`codec.go` 负责保存和加载过滤器。

文件结构大致是：

```text
固定 4096 字节 header
后面跟过滤器数据区
```

header 里保存：

```text
magic
format version
filter kind
hash ID
cell bits
m
k
n
data length
```

有了这些信息，`Load` 才知道：

```text
这是普通 Bloom 还是 Counting Bloom
每个 cell 是 1 bit 还是 4 bit
该用哪个哈希 ID
数据区长度是否正确
```

这也是为什么默认哈希不能随便改。持久化文件只记录 `hashID`，如果同一个 `hashID` 对应的哈希语义变了，旧文件的查询结果就会错。

## 用在用户名查重时的推荐流程

Bloom Filter 不应该单独承担最终一致性约束。用户名注册这种场景，数据库唯一索引仍然必须存在。

注意：下面是**业务系统如何接入本库**的流程，不是本项目内置了数据库功能。本项目只负责第 3 步里的快速判断。

推荐流程：

```text
1. 服务启动时加载 Bloom Filter，里面包含已注册用户名
2. 用户输入 username
3. 查询 Bloom Filter
4. 如果 Bloom Filter 返回 false：
   - 在过滤器数据及时同步的前提下，说明一定不存在
   - 业务系统可以跳过“查重读库”这一步
   - 业务系统继续走注册写入
5. 如果 Bloom Filter 返回 true：
   - 说明可能存在
   - 业务系统查数据库确认
6. 注册成功后：
   - 业务系统写数据库
   - 业务系统把 username 加入 Bloom Filter
```

数据库层仍然要有：

```sql
unique(username)
```

因为 Bloom Filter 解决的是性能问题，不是最终一致性约束。并发注册、延迟同步、服务重启等问题最终都要由权威存储兜底。

## 常见问题

### 1. Bloom Filter 为什么不能回答“一定存在”

因为多个 key 可能设置同一个 bit。某个没有加入过的 key，它对应的所有 bit 也可能刚好被其他 key 设置成 1。

所以：

```text
有 0 -> 一定不存在
全 1 -> 可能存在
```

### 2. 为什么不会有假阴性

只要一个 key 被 Add 过，它对应的所有 bit 都会被设置为 1。普通 Bloom Filter 不会主动把 bit 清零，所以之后查询同一个 key 时，这些 bit 不会变回 0。

前提是：

```text
没有数据损坏
没有错误修改底层存储
使用同一个哈希算法和同一套参数
```

### 3. 这个项目是不是只哈希两次

它只生成两个基础哈希值：

```text
h1, h2
```

但会生成并检查 `k` 个位置：

```text
idx_i = (h1 + i*h2) % m
```

所以不是只检查两个 bit。

更准确地说：

```text
哈希来源是 h1/h2
检查位置数量是 k
```

### 4. 如果 h1/h2 相同会怎样

如果两个 key 的 `h1`、`h2` 完全相同，那么它们生成的所有位置也完全相同。

但这相当于 128 位哈希碰撞，概率极低。

更常见的是：

```text
h1/h2 不同，但取模后的某些位置相同
```

这是正常碰撞。

### 5. k 个位置会不会有重复

理论上会。重复时，实际不同位置数量小于 `k`。

这会略微降低效果，但正常规模下影响通常很小。本项目没有为每个 key 做位置去重，因为去重会增加热路径开销，收益有限。

### 6. m 越大越好吗

在内存允许范围内，`m` 越大，bit 数组越稀疏，假阳性率通常越低。

但 `m` 越大内存越多。Bloom Filter 的本质就是在内存和误判率之间做取舍。

### 7. k 越大越好吗

不是。

`k` 太小，单个 key 设置的位置太少，区分度不够。

`k` 太大，每个 key 设置的位置太多，会更快把 bit 数组填满，反而增加假阳性。

这个项目根据 `n` 和 `p` 自动计算接近最优的 `k`。

### 8. Counting Bloom Filter 为什么最大是 15

因为本项目每个计数器使用 4 bit：

```text
4 bit 最大值是 15
```

这是为了控制内存。如果改成 8-bit counter，Counting Filter 内存会从经典 Bloom 的 4 倍变成 8 倍。

### 9. Counting Bloom Filter 可以随便 Remove 吗

不可以。

只能 Remove 确实 Add 过的元素。删除从未添加过的元素，会把别的 key 共享的计数器减小，可能导致假阴性。

### 10. 这个项目支持自定义哈希吗

当前不支持公开自定义哈希。

内部有 `Hasher` 接口，但放在 `internal/hashing` 包里，不是公开 API。这样可以保持 API 简洁，也避免持久化文件的哈希兼容性问题。

### 11. 为什么热路径不返回 error

`Add`、`MightContain`、`Remove` 是高频操作。这个项目把错误主要放在构造、加载、保存、mmap 打开这些边界上。

热路径不返回 error，可以让调用和性能都更简单。

只读 mmap 上调用写操作会 panic，因为这是调用方明显违反使用约束。

### 12. 是否并发安全

不是。

README 里明确说明这个库非并发安全，调用方需要自己同步。

常见做法：

```text
只读查询：构建完成后多 goroutine 读，通常可接受，但要确保没有并发写
读写混合：调用方用 mutex 或分片锁保护
高并发服务：可以构建不可变快照，定期替换
```

不要在有并发写入时无锁读取同一个过滤器。

## 这个项目的实现取舍

这个项目整体偏向：

```text
API 简洁
热路径快
内存占用低
持久化稳定
依赖少
```

因此它没有暴露太多配置项，例如：

```text
不支持公开自定义哈希
不支持手动指定 m/k
Counting 固定 4-bit counter
默认非并发安全
```

这些不是不能做，而是每加一个配置都会增加 API 面、测试成本和持久化兼容性成本。

对当前项目来说，最核心的能力已经具备：

```text
用 n/p 自动计算参数
用 double hashing 生成 k 个位置
用 bit/nibble 压缩存储
用 Save/Load 做持久化
用 mmap 支持大文件场景
用 Counting 支持删除
```

## 一句话总结

这个项目的核心思想是：

```text
用一个很大的 bit/计数数组表示集合；
每个 key 通过 h1/h2 推导出 k 个位置；
Add 时设置这些位置；
查询时检查这些位置；
只要有一个位置为空，就能确定 key 不存在；
如果全部位置都被占用，只能说 key 可能存在；
用可控的假阳性换取极低内存和极快查询。
```

这就是它能够用于“毫秒级甚至更快地排除大量不存在用户名”的原因。

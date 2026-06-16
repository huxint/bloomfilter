# bloomfilter

[English](README.md) | **简体中文**

一个生产可用的通用 Go 成员判断库：经典 **Bloom Filter** 与 **Counting Bloom
Filter**（支持 `Remove`），带二进制序列化和 `mmap` 持久化，面向数十亿级数据集。

> 非并发安全 —— 并发由调用方自行同步（与内置 `map` 相同的取舍）。

## 安装

```bash
go get github.com/huxint/bloomfilter
```

## 快速上手

```go
f, _ := bloomfilter.New(1_000_000, 0.001) // 100 万元素，0.1% 假阳性率
f.AddString("alice")

if !f.MightContainString("bob") {
    // 一定可用 —— 无需查数据库
}
```

## 原理

Bloom Filter 是一个位数组加 *k* 个哈希函数。它**没有假阴性**（回答“不存在”一定正确），
并有可调的**假阳性**率。因此非常适合做*负缓存*：绝大多数“是否被占用”的查询在亚微秒级
就能给出答案、完全不碰权威存储，只有一小部分（且比例可控）才回查真实数据。

Counting 变体每格存一个 4 位计数器（而非单个比特），因此额外支持 `Remove`
（例如账号注销时释放用户名）。

## API

| 函数 | 用途 |
|---|---|
| `New(n, p) (*BloomFilter, error)` | 内存经典过滤器，容量 `n`、假阳性率 `p` |
| `NewCounting(n, p) (*CountingFilter, error)` | 内存 Counting 过滤器（支持 `Remove`） |
| `CreateMmap(path, kind, n, p) (MmapFilter, error)` | 构建放不下内存的文件级过滤器 |
| `OpenMmap(path, readOnly) (MmapFilter, error)` | 映射既有过滤器文件（`readOnly` → 只读查询） |
| `Save(f, path)` / `Load(path)` | 序列化到磁盘 / 从磁盘加载 |

过滤器方法：`Add`、`MightContain`、`AddString`、`MightContainString`、
`AddedCount`、`Params`，外加 `Remove`（Counting）、`EstimateCardinality` /
`EstimateFalsePositiveRate`（经典）。两者都实现了 `encoding.BinaryMarshaler`、
`io.WriterTo` / `io.ReaderFrom`；mmap 过滤器还提供 `Sync` / `Close`。

热路径方法（`Add` / `MightContain` / `Remove`）不返回 error、零分配。构造和 I/O
才返回 error；损坏或截断的文件会以错误形式上报（`ErrBadMagic`、`ErrVersion`、
`ErrCorrupt`……），**绝不 panic**。

## 实战示例（见 `examples/`）

- **username** —— 用户名/邮箱查重（负缓存 + 数据库兜底 + 用 Counting 过滤器释放）
- **crawler** —— 爬虫 URL 去重，靠 `Save`/`Load` 跨重启保留
- **cacheguard** —— 缓存穿透防护：对一定不存在的 key 直接跳过查库
- **blocklist** —— 弱口令/恶意 URL 黑名单，用只读 `mmap` 加载预建文件

## 持久化

```go
bloomfilter.Save(f, "f.blmf")      // 序列化到磁盘
g, _ := bloomfilter.Load("f.blmf") // 重新加载进内存

// 或对重启时重建代价过高的大文件用 mmap：
mf, _ := bloomfilter.CreateMmap("big.blmf", bloomfilter.KindBloom, 10_000_000_000, 0.001)
defer mf.Close()
ro, _ := bloomfilter.OpenMmap("big.blmf", true) // 只读，用于查询服务
defer ro.Close()
```

`mmap` 依赖 `golang.org/x/sys` —— 这是标准库之外唯一的依赖。

## 内存估算（经典 Bloom；Counting Bloom 为 4 倍）

每元素位数 ≈ `-ln p / (ln 2)²`。

| n | p=1% (k≈7) | p=0.1% (k≈10) | p=0.01% (k≈13) |
|---|---|---|---|
| 1e8（1 亿） | ≈ 120 MB | ≈ 180 MB | ≈ 240 MB |
| 1e9（10 亿）| ≈ 1.2 GB | ≈ 1.8 GB | ≈ 2.4 GB |
| 1e10（100 亿）| ≈ 12 GB | ≈ 18 GB | ≈ 24 GB |

## 说明

- Counting 过滤器用 4 位计数器，在 15 处饱和（已知限制）。只 `Remove` 确实 `Add` 过的元素。
- 默认哈希为 FNV-1a-128 加 splitmix64 finalizer，分布均匀且确定性，持久化后能原样重载。
- 通过 build tag 隔离的 mmap 代码支持 Windows（CI 用 `GOOS=windows` 交叉编译验证）。

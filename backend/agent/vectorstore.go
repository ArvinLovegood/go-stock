package agent

// vectorstore.go — 向量库存储布局与旧版数据迁移。
//
// 目录布局（v2，启用压缩并按用途拆分）：
//
//	memory/.vectorstore/
//	  ltm/                 长期记忆 qa_history（独立 chromem 持久化库，gob.gz 压缩）
//	  kb/                  自定义知识库（独立 chromem 持久化库，gob.gz 压缩）
//	  .kb_meta.json        KB 元信息（位置不变，knowledge_base.go 管理）
//	  .kb_graph_*.json     知识图谱（位置不变，knowledge_graph.go 管理）
//
// 旧版布局：所有 collection 直接放在 .vectorstore/<hash>/ 下（未压缩 .gob），
// 长期记忆与知识库共用同一个 chromem.DB 实例。
//
// 背景与动机：
//   - chromem-go 的 NewPersistentDB 初始化时同步、串行加载全部文档（上游 TODO），
//     数据量大（数万文件 / 数百 MB）时启动显著变慢；
//   - compress=true 时文档以 gob.gz 持久化，float32 向量文本压缩比高，
//     启动加载的磁盘读取量约降一个数量级，Windows Defender 扫描开销同步下降；
//   - 拆分后长期记忆与知识库各自独立实例，互不拖累（重建/重置互不影响）。
//
// 旧数据迁移：首次初始化时自动执行一次性迁移——并行解码旧 .gob 文档，
// 以预计算 embedding 写入新目录（gzip 压缩），原目录移入
// memory/.vectorstore-backup-<时间戳>/ 备份。迁移幂等，失败后下次启动重试。

import (
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/philippgille/chromem-go"

	"go-stock/backend/logger"
)

const (
	// vectorStoreLTMDirName 长期记忆持久化子目录（位于 .vectorstore 下）
	vectorStoreLTMDirName = "ltm"
	// vectorStoreKBDirName 知识库持久化子目录（位于 .vectorstore 下）
	vectorStoreKBDirName = "kb"
	// legacyCollectionMetadataFile chromem-go collection 元数据文件名（未压缩格式）
	legacyCollectionMetadataFile = "00000000.gob"
	// migrationMaxConcurrency 迁移时并行解码/写入的协程数上限
	migrationMaxConcurrency = 8
)

var (
	vectorStoresMu   sync.Mutex
	vectorStoreLTMDB *chromem.DB
	vectorStoreKBDB  *chromem.DB
	vectorStoresErr  error
)

// hash2hex 与 chromem-go 内部实现一致：sha256 前 4 字节的 hex 编码，
// 用于从 collection 名推导持久化子目录名（文档扫描、保留策略清理需要）。
func hash2hex(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:4])
}

// vectorStoreRootDir 向量库根目录 <exe_dir>/memory/.vectorstore/。
// 根目录为空时返回空串（调用方视为初始化失败）。
func vectorStoreRootDir() string {
	root := deepAgentRootDir()
	if root == "" || root == "." {
		return ""
	}
	return filepath.Join(root, memoryDirName, vectorStoreDirName)
}

// ltmStorePath 长期记忆持久化库目录
func ltmStorePath() string {
	return filepath.Join(vectorStoreRootDir(), vectorStoreLTMDirName)
}

// kbStorePath 知识库持久化库目录
func kbStorePath() string {
	return filepath.Join(vectorStoreRootDir(), vectorStoreKBDirName)
}

// ensureVectorStores 确保长期记忆与知识库两个持久化向量库已就绪（含旧数据迁移）。
//
// 幂等：两个库均已创建时直接返回缓存错误状态（可能为 nil）。
// 任一库创建失败时保留已成功部分，下次调用仅重试失败部分。
func ensureVectorStores() error {
	vectorStoresMu.Lock()
	defer vectorStoresMu.Unlock()
	if vectorStoreLTMDB != nil && vectorStoreKBDB != nil {
		return vectorStoresErr
	}

	root := vectorStoreRootDir()
	if root == "" {
		vectorStoresErr = fmt.Errorf("无法确定向量库根目录（deepAgentRootDir 为空）")
		return vectorStoresErr
	}

	// 旧版未压缩数据一次性迁移（幂等；失败不阻断，旧目录保留待下次重试）
	if err := migrateLegacyVectorStore(root); err != nil {
		logger.SugaredLogger.Errorf("旧版向量库迁移失败（旧数据保留原目录，下次启动重试）: %v", err)
	}

	var firstErr error
	if vectorStoreLTMDB == nil {
		db, err := chromem.NewPersistentDB(ltmStorePath(), true /* compress */)
		if err != nil {
			firstErr = fmt.Errorf("长期记忆向量库初始化失败: %w", err)
			logger.SugaredLogger.Errorf("%v (path=%s)", firstErr, ltmStorePath())
		} else {
			vectorStoreLTMDB = db
		}
	}
	if vectorStoreKBDB == nil {
		db, err := chromem.NewPersistentDB(kbStorePath(), true /* compress */)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("知识库向量库初始化失败: %w", err)
			}
			logger.SugaredLogger.Errorf("知识库向量库初始化失败: %v (path=%s)", err, kbStorePath())
		} else {
			vectorStoreKBDB = db
		}
	}
	vectorStoresErr = firstErr
	return vectorStoresErr
}

// resetLTMVectorDB 重置长期记忆持久化库单例（切换向量服务后调用）。
//
// 背景：chromem-go 的 GetCollection 在 collection 已加载且 embed 非空时，
// 会忽略新传入的 embeddingFunc——单例一旦用旧配置初始化，配置变更后本会话内
// 永远不会生效。重置后下次 ensureVectorStores 会以新配置重建 DB 实例；
// DB 为持久化存储（compress=true），重建时自动从磁盘重新加载文档。
// 知识库使用独立实例，不受影响。
func resetLTMVectorDB() {
	vectorStoresMu.Lock()
	defer vectorStoresMu.Unlock()
	vectorStoreLTMDB = nil
	vectorStoresErr = nil
}

// ============ 旧版数据迁移 ============

// migrateLegacyVectorStore 将旧版布局（collection 目录直接位于 root 下、未压缩 .gob）
// 迁移到 v2 布局（ltm/ 与 kb/ 子目录、gob.gz 压缩）。
//
// 迁移步骤（幂等）：
//  1. 识别 root 下含 00000000.gob 元数据文件的 collection 目录（ltm/kb/隐藏目录除外）
//  2. 并行解码旧文档（保留预计算 embedding，不重新调用向量服务）
//  3. 以 compress=true 写入目标目录（qa_history → ltm/，其余 → kb/）
//  4. 原目录 rename 到 memory/.vectorstore-backup-<时间戳>/ 备份
//
// 任一 collection 迁移失败即中止（已完成的保留，原目录不动，下次启动重试）。
func migrateLegacyVectorStore(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var legacyDirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == vectorStoreLTMDirName || name == vectorStoreKBDirName || strings.HasPrefix(name, ".") {
			continue
		}
		dir := filepath.Join(root, name)
		if fileExists(filepath.Join(dir, legacyCollectionMetadataFile)) {
			legacyDirs = append(legacyDirs, dir)
		}
	}
	if len(legacyDirs) == 0 {
		return nil
	}

	logger.SugaredLogger.Infof("检测到旧版未压缩向量库（%d 个 collection），开始一次性迁移（并行解码 → gzip 压缩写入，不重新请求向量服务）", len(legacyDirs))
	backupDir := filepath.Join(filepath.Dir(root), fmt.Sprintf(".vectorstore-backup-%s", time.Now().Format("20060102-150405")))

	migrated := 0
	for _, dir := range legacyDirs {
		collName, collMeta, err := readLegacyCollectionMeta(dir)
		if err != nil || collName == "" {
			logger.SugaredLogger.Warnf("跳过无法识别的旧 collection 目录 %s: %v", dir, err)
			continue
		}

		target := kbStorePath()
		if collName == longTermMemoryCollectionName {
			target = ltmStorePath()
		}

		docs, err := decodeLegacyDocsParallel(dir)
		if err != nil {
			return fmt.Errorf("解码 collection %q 失败: %w", collName, err)
		}
		if err := writeCompressedCollection(target, collName, collMeta, docs); err != nil {
			return fmt.Errorf("写入 collection %q 失败: %w", collName, err)
		}

		// 原目录移入备份（同盘 rename，瞬时完成；失败仅告警——重试时会按 ID 覆盖，无数据风险）
		if mkErr := os.MkdirAll(backupDir, 0o700); mkErr == nil {
			if rmErr := os.Rename(dir, filepath.Join(backupDir, filepath.Base(dir))); rmErr != nil {
				logger.SugaredLogger.Warnf("旧 collection 目录移入备份失败（迁移结果不受影响）: %v", rmErr)
			}
		}

		migrated++
		logger.SugaredLogger.Infof("collection %q 迁移完成: %d 条文档 → %s", collName, len(docs), target)
	}

	if migrated > 0 {
		logger.SugaredLogger.Infof("旧版向量库迁移完成: %d 个 collection，原目录已备份至 %s（确认功能正常后可手动删除）", migrated, backupDir)
	}
	return nil
}

// readLegacyCollectionMeta 读取旧 collection 目录的元数据文件（gob，未压缩），
// 返回 collection 名称与元信息。
func readLegacyCollectionMeta(dir string) (string, map[string]string, error) {
	f, err := os.Open(filepath.Join(dir, legacyCollectionMetadataFile))
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = f.Close() }()

	// gob 按字段名结构化解码，与 chromem-go 持久化时的匿名 struct 兼容
	var meta struct {
		Name     string
		Metadata map[string]string
	}
	if err := gob.NewDecoder(f).Decode(&meta); err != nil {
		return "", nil, err
	}
	return meta.Name, meta.Metadata, nil
}

// decodeLegacyDocsParallel 并行解码旧 collection 目录下的所有文档 .gob 文件。
//
// 单个文件解码失败仅告警跳过（不中止迁移），全部失败时返回错误。
// 返回文档保持 embedding 原值（已归一化），写入新库时不会重新请求向量服务。
func decodeLegacyDocsParallel(dir string) ([]chromem.Document, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || e.Name() == legacyCollectionMetadataFile {
			continue
		}
		if strings.HasSuffix(e.Name(), ".gob") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	if len(files) == 0 {
		return nil, nil
	}

	docs := make([]chromem.Document, len(files))
	skipped := 0
	var (
		mu     sync.Mutex
		next   int
		wg     sync.WaitGroup
		failed int
	)
	worker := func() {
		defer wg.Done()
		for {
			mu.Lock()
			i := next
			next++
			mu.Unlock()
			if i >= len(files) {
				return
			}
			doc, err := decodeLegacyDocFile(files[i])
			if err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				logger.SugaredLogger.Warnf("迁移: 解码文档失败，跳过 %s: %v", filepath.Base(files[i]), err)
				continue
			}
			docs[i] = *doc
		}
	}
	conc := runtime.NumCPU()
	if conc > migrationMaxConcurrency {
		conc = migrationMaxConcurrency
	}
	if conc < 2 {
		conc = 2
	}
	wg.Add(conc)
	for i := 0; i < conc; i++ {
		go worker()
	}
	wg.Wait()

	if failed == len(files) {
		return nil, fmt.Errorf("全部 %d 个文档解码失败", len(files))
	}
	if failed > 0 {
		skipped = failed
		logger.SugaredLogger.Warnf("迁移: %d 个文档解码失败被跳过", skipped)
	}
	// 压缩掉解码失败留下的空位（ID 为空）
	result := make([]chromem.Document, 0, len(docs)-failed)
	for _, d := range docs {
		if d.ID != "" {
			result = append(result, d)
		}
	}
	return result, nil
}

// decodeLegacyDocFile 解码单个旧文档 .gob 文件（未压缩 gob，结构与 chromem-go 一致）
func decodeLegacyDocFile(path string) (*chromem.Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var doc chromem.Document
	if err := gob.NewDecoder(f).Decode(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// writeCompressedCollection 将 collection（含元信息与全部文档）以 compress=true
// 写入目标持久化库目录。文档携带预计算 embedding，AddDocuments 不会重新请求向量服务。
func writeCompressedCollection(targetDir, collName string, collMeta map[string]string, docs []chromem.Document) error {
	db, err := chromem.NewPersistentDB(targetDir, true /* compress */)
	if err != nil {
		return err
	}
	// embedFunc 传 nil 安全：所有文档均已带 embedding，AddDocument 仅在 embedding
	// 为空时才调用 embed 函数；此临时实例不做 Query。
	coll, err := db.GetOrCreateCollection(collName, collMeta, nil)
	if err != nil {
		return err
	}
	if len(docs) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	conc := runtime.NumCPU()
	if conc > migrationMaxConcurrency {
		conc = migrationMaxConcurrency
	}
	if conc < 2 {
		conc = 2
	}
	return coll.AddDocuments(ctx, docs, conc)
}

// fileExists 判断文件是否存在
func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// Command build-demo generates the demo evaluation dataset used by WP2.
//
// Usage:
//
//	go run ./dataset [output-dir]
//
// The default output directory is dataset/demo.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/parquet-go/parquet-go"
)

type textInfo struct {
	ID   int64  `parquet:"id"`
	Text string `parquet:"text"`
}

type relsInfo struct {
	QID int64 `parquet:"qid"`
	PID int64 `parquet:"pid"`
}

type qaInfo struct {
	QID int64 `parquet:"qid"`
	AID int64 `parquet:"aid"`
}

func main() {
	dir := "dataset/demo"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}

	queries := []textInfo{
		{1, "什么是向量检索？"},
		{2, "混合检索相比纯向量检索有什么优势？"},
		{3, "为什么需要重排（Rerank）？"},
		{4, "WeKnora 支持哪些向量数据库？"},
		{5, "什么是知识库的 chunk？"},
		{6, "如何提高检索准确率？"},
		{7, "评估 RAG 系统时为什么需要数据集？"},
		{8, "BLEU 指标衡量什么？"},
		{9, "ROUGE 指标衡量什么？"},
		{10, "NDCG 是什么？"},
		{11, "什么是 grounding 答案？"},
		{12, "WeKnora 的评测流程是怎样的？"},
		{13, "为什么要固定数据集来对比模型？"},
		{14, "检索召回率和准确率有什么区别？"},
		{15, "RAG 系统包含哪些核心组件？"},
	}

	corpus := []textInfo{
		{1, "向量检索把文本通过 embedding 模型映射到高维向量空间，再用余弦相似度等度量找出语义最接近的片段。"},
		{2, "混合检索同时使用向量相似度与关键词匹配，兼顾语义相关性与字面匹配，通常能提升召回率。"},
		{3, "重排模型会对初步检索结果重新打分排序，把真正相关的文档排在前面，从而提升最终答案质量。"},
		{4, "WeKnora 支持 Qdrant、Milvus 等向量数据库，也支持 SQLite 内置的轻量向量检索，便于本地开发。"},
		{5, "知识库的 chunk 是文档切分后的最小索引单元，合理的 chunk 大小与重叠设置会影响检索效果。"},
		{6, "提高检索准确率可以调整 TopK、阈值、分块策略，或接入重排模型并固定评测数据集反复对比。"},
		{7, "评测数据集包含问题、参考答案和相关语料，是衡量 RAG 效果、对比模型和参数变化的统一标尺。"},
		{8, "BLEU 通过 n-gram 重叠度衡量生成文本与参考答案的相似程度，常用于机器翻译和生成质量评估。"},
		{9, "ROUGE 以召回为导向，衡量生成文本中覆盖参考答案内容的比例，常用 ROUGE-1、ROUGE-2、ROUGE-L。"},
		{10, "NDCG 是归一化折损累计增益，奖励相关文档排在检索结果前面，是评估排序质量的常用指标。"},
		{11, "grounding 答案指模型只依据检索到的知识库内容回答，不依赖训练数据中的记忆，便于溯源与审计。"},
		{12, "WeKnora 评测会创建临时知识库并灌入数据集语料，逐个问题运行检索与生成链路，最后汇总 12 项指标。"},
		{13, "固定数据集后，两次运行结果的差异才能归因于模型、参数或代码变化，否则无法得出可信结论。"},
		{14, "准确率衡量检索结果中有多少是相关的，召回率衡量所有相关文档中有多少被检索到，两者侧重点不同。"},
		{15, "RAG 系统通常包含文档解析、知识库索引、向量检索、重排、生成和评测反馈等核心组件。"},
	}

	answers := []textInfo{
		{1, "向量检索是把文本映射到高维向量空间，通过相似度计算找出语义最接近的内容片段。"},
		{2, "混合检索结合向量相似度与关键词匹配，既能找语义相近的内容，也能保证字面关键词命中，通常召回更好。"},
		{3, "初步检索结果可能包含噪声，重排模型重新打分后能把相关文档排到前面，提高最终回答的准确性。"},
		{4, "WeKnora 支持 Qdrant、Milvus 等向量数据库，本地 Lite 模式使用 SQLite 内置向量检索。"},
		{5, "chunk 是文档切分后的最小索引单元，合理设置大小与重叠可以提升检索效果。"},
		{6, "可以通过调整 TopK、阈值、分块策略，并接入重排模型来提高检索准确率，同时用固定数据集验证效果。"},
		{7, "评测数据集是问题、答案和相关语料的集合，用来统一衡量 RAG 效果并对比不同配置。"},
		{8, "BLEU 衡量生成文本与参考答案在 n-gram 层面的重叠程度，值越高表示越接近参考答案。"},
		{9, "ROUGE 衡量生成文本对参考答案内容的覆盖程度，包含 ROUGE-1、ROUGE-2 和 ROUGE-L 等变体。"},
		{10, "NDCG 根据相关文档在排序结果中的位置计算折损累计增益，越靠前的相关文档得分越高。"},
		{11, "grounding 答案只依据检索到的知识库内容生成，不依赖模型训练记忆，保证回答可溯源。"},
		{12, "WeKnora 评测会临时建库、灌入语料，逐题跑检索和生成，最后输出检索与生成两组指标。"},
		{13, "固定数据集能让不同运行之间可对比，避免数据集变化干扰模型或参数的效果判断。"},
		{14, "准确率看检索结果中有多少相关，召回率看相关文档被找到多少，两者需要结合使用。"},
		{15, "RAG 系统的核心组件包括文档解析、索引、检索、重排、生成与评测反馈。"},
	}

	qrels := make([]relsInfo, 0, len(queries))
	qas := make([]qaInfo, 0, len(queries))
	for _, q := range queries {
		qrels = append(qrels, relsInfo{QID: q.ID, PID: q.ID})
		qas = append(qas, qaInfo{QID: q.ID, AID: q.ID})
	}

	writeFiles(dir, queries, corpus, answers, qrels, qas)

	fmt.Printf("Generated demo dataset in %s\n", dir)
	fmt.Printf("queries=%d corpus=%d answers=%d qrels=%d qas=%d\n",
		len(queries), len(corpus), len(answers), len(qrels), len(qas))
}

func writeFiles(
	dir string,
	queries []textInfo,
	corpus []textInfo,
	answers []textInfo,
	qrels []relsInfo,
	qas []qaInfo,
) {
	writeParquet(dir, "queries.parquet", queries)
	writeParquet(dir, "corpus.parquet", corpus)
	writeParquet(dir, "answers.parquet", answers)
	writeParquet(dir, "qrels.parquet", qrels)
	writeParquet(dir, "qas.parquet", qas)
}

func writeParquet[T any](dir, name string, rows []T) {
	path := filepath.Join(dir, name)
	if err := parquet.WriteFile(path, rows); err != nil {
		panic(fmt.Sprintf("write %s: %v", path, err))
	}
}

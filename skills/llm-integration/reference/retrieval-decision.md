# Retrieval decision — prompt, retrieval or fine-tune

The question is never "should we add RAG?" but **"what kind of failure are we fixing?"**

## Decide by failure type

| Failure you observe | Cause | Fix |
|---|---|---|
| Model does not know the fact / fact is stale | **information** | retrieval (p.336) |
| Model knows but answers in wrong format, wrong tone, wrong level | **behavior** | prompt first, then fine-tune (p.336) |
| Model ignores a rule that appears in the prompt | behavior, or context ordering | move the rule to the edges of the context (p.238-239) |
| Model mixes up which customer/document it is talking about | identity/grounding | retrieval with explicit metadata, not a bigger window |
| Model is too slow / too expensive | budget | smaller model, cache, retrieval narrower — not fine-tune |

Do not fine-tune as a first move: it hides prompt defects, improves one task while degrading others, and locks the model version (p.333).

The measured comparison matters more than the intuition: a small model with retrieval went **0.481 → 0.875** on the same task, while fine-tuning the same model gave **0.504** (p.337). Retrieval fixed the failure; fine-tuning polished around it.

## Prompt vs retrieval: the cheap test

Before building an index, ask how big the corpus is:

- Corpus fits comfortably (e.g. tens of thousands of tokens) and changes slowly → include it whole. Long context is cheaper than a pipeline you must operate (p.275).
- Corpus grows, or the model "focuses on the wrong part" → retrieval, which also cuts cost per query (p.275).
- "Long context kills RAG" is not a rule: there will always be applications needing more context than any window, and every extra context token costs money (p.275).

## Hybrid retrieval, cheap stage first

1. **Hybrid search**: term-based + embedding, fused (RRF). Embeddings obscure exact keywords — error codes, IDs, product names — and those are precisely what users search (p.284).
2. **Rerank cheap → expensive**: broad first stage, precise second stage over a small candidate set. Reranking everything defeats the purpose (p.286).
3. **Rewrite the query before retrieving** in multi-turn: turn the last message into a self-contained query. If the rewrite needs information that does not exist, say so instead of inventing (p.290).

## Chunking by measurement

- Split on structure (heading, function, ticket boundary), not on a fixed character count.
- Overlap enough that a sentence split by the boundary is still recoverable — overlap is the cheapest fix for lost context and the most common missing setting (p.275).
- Enrich each chunk with its title/summary so an orphan chunk is still retrievable (p.290).
- Measure `context precision` and `context recall` on the retriever **in isolation**: if you only measure the final answer, you cannot tell whether the error came from retrieval or from the model (p.284).

## What retrieval does not fix

- Bad prompt structure (`prompt-engineering`).
- Output format instability (that is `output-contract.md` in this skill, plus prompt work).
- Latency and cost at scale without cache (`integration-boundary.md#cache`).

## Checklist

- [ ] The failure is named as information or behavior (p.336).
- [ ] Prompt/retrieval options were tried before fine-tune (p.333).
- [ ] Corpus size and change rate justified the choice — whole context or retrieval (p.275).
- [ ] Retrieval is hybrid, with a cheap first stage and rerank (p.284, p.286).
- [ ] Multi-turn queries are rewritten to be self-contained (p.290).
- [ ] Chunking follows document structure, with overlap and title enrichment.
- [ ] Context precision and recall are measured on the retriever alone (p.284).
- [ ] If fine-tune is chosen, the target failure, the data volume (50-100 examples first) and the model version are recorded (p.394).

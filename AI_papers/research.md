# AI papers

下面按**年份**梳理大模型（LLM / 基础模型）发展中最关键、必读的论文，兼顾架构、预训练范式、缩放定律、对齐、推理与开源生态。括号内为常见简称。

## 2017｜Transformer 诞生
- **Attention Is All You Need** (Vaswani et al., Google) — 提出 Transformer 自注意力架构，取代 RNN/LSTM，是现代所有大模型基石。

## 2018｜预训练范式确立
- **Improving Language Understanding by Generative Pre-Training** (Radford et al., OpenAI) — GPT-1，decoder-only 生成式预训练 + 微调。
- **BERT: Pre-training of Deep Bidirectional Transformers…** (Devlin et al., Google) — 双向 Encoder + MLM，开启"预训练+微调"浪潮。

## 2019｜生成式扩展与统一框架
- **Language Models are Unsupervised Multitask Learners** (Radford et al., OpenAI) — GPT-2（1.5B），展示零样本多任务能力。
- **RoBERTa** (Liu et al., Meta) — 证明 BERT 训练不足，优化训练策略显著提效。
- **T5: Exploring the Limits of Transfer Learning with a Unified Text-to-Text Transformer** (Raffel et al., Google) — 所有任务统一为 text-to-text。
- **XLNet** (Yang et al., Google/CMU) — 排列语言建模，融合 AR 与 AE 优点。

## 2020｜规模化与少样本学习
- **Scaling Laws for Neural Language Models** (Kaplan et al., OpenAI) — 参数量/数据/算力的幂律缩放规律。
- **Language Models are Few-Shot Learners** (Brown et al., OpenAI) — GPT-3（175B），in-context learning 涌现。
- **ELECTRA** (Clark et al., Stanford/Google) — replaced-token detection，更高效预训练。
- **Retrieval-Augmented Generation for Knowledge-Intensive NLP Tasks** (Lewis et al.) — RAG 框架。

## 2021｜MoE、指令微调与多模态
- **Switch Transformers** (Fedus et al., Google) — 万亿参数稀疏 MoE。
- **GLaM** (Du et al., Google) — 更高效的 MoE 缩放。
- **FLAN: Finetuned Language Models are Zero-Shot Learners** (Wei et al., Google) — 指令微调提升零样本泛化。
- **LoRA: Low-Rank Adaptation of Large Language Models** (Hu et al.) — 低秩适配，PEFT 起点。
- **RoFormer / Rotary Position Embedding (RoPE)** (Su et al.) — 旋转位置编码，成现代标配。
- **CLIP** (Radford et al., OpenAI) — 图文对比学习，多模态基座。
- **Codex: Evaluating Large Language Models Trained on Code** (Chen et al., OpenAI) — 代码生成（Copilot 内核）。

## 2022｜对齐、CoT、Chinchilla
- **Training language models to follow instructions with human feedback (InstructGPT)** (Ouyang et al., OpenAI) — RLHF 三阶段 SFT→RM→PPO，ChatGPT 前身。
- **Training Compute-Optimal Large Language Models (Chinchilla)** (Hoffmann et al., DeepMind) — 参数量与数据需同步缩放，推翻 Kaplan 结论。
- **Chain-of-Thought Prompting Elicits Reasoning in LLMs** (Wei et al., Google) — CoT 推理。
- **Emergent Abilities of Large Language Models** (Wei et al.) — 涌现能力系统论述。
- **PaLM** (Chowdhery et al., Google) — 540B 稠密模型。
- **FlashAttention** (Dao et al.) — IO-aware 精确注意力，训练加速标配。
- **Constitutional AI** (Bai et al., Anthropic) — RLAIF，用原则替代部分人类标注。
- **ReAct** (Yao et al.) — 推理+行动交错，Agent 蓝图。
- **OPT / BLOOM** — Meta 与 BigScience 开放百亿级基线，推动开源。

## 2023｜开源革命与偏好优化
- **LLaMA: Open and Efficient Foundation Language Models** (Touvron et al., Meta) — 7~65B 开放权重，引爆开源生态。
- **Llama 2** (Touvron et al., Meta) — 可商用开放权重 + Chat 版本。
- **Direct Preference Optimization (DPO)** (Rafailov et al.) — 无需 RL 的直接偏好优化。
- **Mistral 7B** (Jiang et al.) — GQA + 滑动窗口，小模型效率标杆。
- **Toolformer** (Schick et al.) — 模型自学调用 API。
- **GPT-4 Technical Report** (OpenAI) — 多模态前沿模型。
- **LLaVA** (Liu et al.) — 开源多模态指令微调起点。

## 2024｜MoE 普及、长上下文与推理模型
- **Mixtral of Experts** (Jiang et al., Mistral) — 8×7B 稀疏 MoE 开源。
- **The Llama 3 Herd of Models** (Dubey et al., Meta) — Llama 3 405B 系列。
- **DeepSeek-V3** (DeepSeek) — 671B MoE + FP8 + 辅助无损负载均衡。
- **BitNet b1.58** (Ma et al.) — 1.58-bit 量化训练。
- **Mamba-2** (Dao & Gu) — 状态空间模型 SSD 对偶形式。
- OpenAI **o1** 体系（博客为主）— 测试时计算（test-time compute）缩放推理。

## 2025｜纯 RL 推理开源化
- **DeepSeek-R1** (DeepSeek) — R1-Zero 纯 RL 激发推理 + GRPO + 冷启动蒸馏，开源推理模型标杆。
- **Titans** (Behrouz et al.) — 神经网络长期记忆机制。

> 脉络一句话：**2017 Transformer → 2018-2019 BERT/GPT 预训练 → 2020 GPT-3+Scaling → 2022 RLHF/Chinchilla/CoT → 2023 LLaMA 开源 → 2024 MoE/效率 → 2025 推理模型 RL 化**。


## 一、架构（Architecture）
- [Attention Is All You Need](https://arxiv.org/abs/1706.03762) — 2017 · Transformer · arXiv:1706.03762
- [BERT: Pre-training of Deep Bidirectional Transformers for Language Understanding](https://arxiv.org/abs/1810.04805) — 2018 · BERT · arXiv:1810.04805
- [T5: Exploring the Limits of Transfer Learning with a Unified Text-to-Text Transformer](https://arxiv.org/abs/1910.10683) — 2019 · T5 · arXiv:1910.10683
- [Switch Transformers: Scaling to Trillion Parameter Models with Simple and Sparse Mixture of Experts](https://arxiv.org/abs/2101.03961) — 2021 · Switch · arXiv:2101.03961
- [FlashAttention: Fast and Memory-Efficient Exact Attention with IO-Awareness](https://arxiv.org/abs/2205.14135) — 2022 · FlashAttention · arXiv:2205.14135
- [Mistral 7B](https://arxiv.org/abs/2310.06825) — 2023 · Mistral · arXiv:2310.06825
- [Mixtral of Experts](https://arxiv.org/abs/2401.04088) — 2024 · Mixtral · arXiv:2401.04088
- [BitNet: Scaling 1-bit Transformers for Large Language Models](https://arxiv.org/abs/2402.17764) — 2024 · BitNet b1.58 · arXiv:2402.17764

## 二、训练（Pre-training / Scaling）
- [Improving Language Understanding by Generative Pre-Training](https://openai.com/research/language-unsupervised) — 2018 · GPT-1 · OpenAI 技术报告（无 arXiv）
- [Language Models are Unsupervised Multitask Learners](https://openai.com/research/better-language-models) — 2019 · GPT-2 · OpenAI 技术报告（无 arXiv）
- [Scaling Laws for Neural Language Models](https://arxiv.org/abs/2001.08361) — 2020 · Kaplan Scaling · arXiv:2001.08361
- [Language Models are Few-Shot Learners](https://arxiv.org/abs/2005.14165) — 2020 · GPT-3 · arXiv:2005.14165
- [LoRA: Low-Rank Adaptation of Large Language Models](https://arxiv.org/abs/2106.09685) — 2021 · LoRA · arXiv:2106.09685
- [Training Compute-Optimal Large Language Models](https://arxiv.org/abs/2203.15556) — 2022 · Chinchilla · arXiv:2203.15556
- [Retrieval-Augmented Generation for Knowledge-Intensive NLP Tasks](https://arxiv.org/abs/2005.11401) — 2020 · RAG · arXiv:2005.11401
- [DeepSeek-V3 Technical Report](https://arxiv.org/abs/2412.19437) — 2024 · DS-V3 · arXiv:2412.19437

## 三、对齐（Alignment / RLHF）
- [Training Language Models to Follow Instructions with Human Feedback](https://arxiv.org/abs/2203.02155) — 2022 · InstructGPT / RLHF · arXiv:2203.02155
- [Constitutional AI: Harmlessness from AI Feedback](https://arxiv.org/abs/2212.08073) — 2022 · CAI / RLAIF · arXiv:2212.08073
- [Direct Preference Optimization: Your Language Model is Secretly a Reward Model](https://arxiv.org/abs/2305.18290) — 2023 · DPO · arXiv:2305.18290

## 四、推理（Reasoning / Agent）
- [Chain-of-Thought Prompting Elicits Reasoning in Large Language Models](https://arxiv.org/abs/2201.11903) — 2022 · CoT · arXiv:2201.11903
- [Self-Consistency Improves Chain of Thought Reasoning in Language Models](https://arxiv.org/abs/2203.11171) — 2022 · SC-CoT · arXiv:2203.11171
- [ReAct: Synergizing Reasoning and Acting in Language Models](https://arxiv.org/abs/2210.03629) — 2022 · ReAct · arXiv:2210.03629
- [Toolformer: Language Models Can Teach Themselves to Use Tools](https://arxiv.org/abs/2302.04761) — 2023 · Toolformer · arXiv:2302.04761
- [DeepSeek-R1: Incentivizing Reasoning Capability in LLMs via Reinforcement Learning](https://arxiv.org/abs/2501.12948) — 2025 · R1 / R1-Zero · arXiv:2501.12948

## 五、开源生态（Open-weight）
- [LLaMA: Open and Efficient Foundation Language Models](https://arxiv.org/abs/2302.13971) — 2023 · LLaMA-1 · arXiv:2302.13971
- [Llama 2: Open Foundation and Fine-Tuned Chat Models](https://arxiv.org/abs/2307.09288) — 2023 · LLaMA-2 · arXiv:2307.09288
- [The Llama 3 Herd of Models](https://arxiv.org/abs/2407.21783) — 2024 · Llama 3 · arXiv:2407.21783
- [GPT-4 Technical Report](https://openai.com/research/gpt-4) — 2023 · GPT-4 · OpenAI 技术报告（无 arXiv）
- [CLIP: Learning Transferable Visual Models From Natural Language Supervision](https://openai.com/research/clip) — 2021 · CLIP · OpenAI 技术报告（无 arXiv）

## 极简通读链（10 篇）
1706.03762 → 1810.04805 → 2005.14165 → 2001.08361 → 2203.15556
→ 2203.02155 → 2201.11903 → 2305.18290 → 2302.13971 → 2501.12948
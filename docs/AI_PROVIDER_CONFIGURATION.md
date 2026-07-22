# AI provider configuration

go-stock separates the input context budget from the maximum model output. The
legacy `maxTokens` column remains supported for older configurations, while new
configurations should use `maxInputTokens` and `maxOutputTokens`.

## Recommended profiles

| Model | Provider | Input | Output | Reasoning | Temperature | Timeout |
| --- | --- | ---: | ---: | --- | ---: | ---: |
| GPT-5.6 Sol | OpenAI / GPT | 96,000 | 16,384 | medium | omitted | 900 s |
| Grok 4.5 | xAI / Grok | 96,000 | 8,192 | medium | 0.2 | 600 s |

Use `high` reasoning only for explicit deep-analysis work. Raising the output
limit does not improve answer quality by itself and increases latency and cost.

The direct OpenAI-compatible API remains the default path for streaming, tools,
scheduled jobs, and chat. GPT/Grok reasoning is sent as `reasoning_effort`; the
non-standard `thinking` object is retained only for legacy compatible providers.

## Local Codex deep analysis

The Agent assistant exposes a separate **Codex 深度分析** action. It runs the
installed `codex exec` with:

- an isolated temporary `CODEX_HOME` containing copies of the current auth and
  configuration files;
- `--ephemeral`, a read-only sandbox, and no Git-repository requirement;
- one job at a time, a ten-minute timeout, GPT-5.6 Sol, and medium reasoning;
- no automatic fallback to a paid API provider.

This path is intended for deliberate, long-running analysis. It is not used by
cron tasks or as the normal chat transport.

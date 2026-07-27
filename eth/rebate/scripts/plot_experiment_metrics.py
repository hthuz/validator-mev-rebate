#!/usr/bin/env python3
"""根据 rebate 实验记录生成汇报图表。"""

from __future__ import annotations

import argparse
import json
from collections import Counter, defaultdict
from pathlib import Path
from typing import Iterable

try:
    import matplotlib.pyplot as plt
except ImportError as exc:  # pragma: no cover
    raise SystemExit(
        "缺少 matplotlib，请先执行: python3 -m pip install matplotlib"
    ) from exc


def load_jsonl(path: Path) -> list[dict]:
    if not path.exists():
        return []
    rows: list[dict] = []
    with path.open("r", encoding="utf-8") as handle:
        for line in handle:
            line = line.strip()
            if not line:
                continue
            rows.append(json.loads(line))
    return rows


def wei_to_eth(value: str | int | float | None) -> float:
    if value in (None, "", 0):
        return 0.0
    return int(value) / 10**18


def rolling_success_rate(flags: Iterable[bool], window: int = 20) -> list[float]:
    values = list(flags)
    result: list[float] = []
    for idx in range(len(values)):
        start = max(0, idx - window + 1)
        chunk = values[start : idx + 1]
        result.append(sum(1 for item in chunk if item) / len(chunk))
    return result


def plot_block_profit(blocks: list[dict], output_dir: Path) -> None:
    if not blocks:
        return
    blocks = sorted(blocks, key=lambda item: item["block_number"])
    x = [item["block_number"] for item in blocks]
    mev_profit = [wei_to_eth(item["total_mev_profit_wei"]) for item in blocks]
    refundable = [wei_to_eth(item["total_refundable_wei"]) for item in blocks]

    plt.figure(figsize=(10, 5))
    plt.plot(x, mev_profit, marker="o", label="MEV profit (ETH)")
    plt.plot(x, refundable, marker="s", label="Refundable value (ETH)")
    plt.xlabel("Block number")
    plt.ylabel("ETH")
    plt.title("Block-level MEV Profit and Refundable Value")
    plt.grid(alpha=0.3)
    plt.legend()
    plt.tight_layout()
    plt.savefig(output_dir / "block_profit_refund.png", dpi=180)
    plt.close()


def plot_block_success(blocks: list[dict], output_dir: Path) -> None:
    if not blocks:
        return
    blocks = sorted(blocks, key=lambda item: item["block_number"])
    x = [item["block_number"] for item in blocks]
    success_rate = [item.get("success_rate", 0.0) for item in blocks]
    bundle_count = [item.get("bundle_count", 0) for item in blocks]

    fig, ax1 = plt.subplots(figsize=(10, 5))
    ax1.plot(x, success_rate, color="#1f77b4", marker="o", label="Success rate")
    ax1.set_xlabel("Block number")
    ax1.set_ylabel("Success rate")
    ax1.set_ylim(0, 1.05)
    ax1.grid(alpha=0.3)

    ax2 = ax1.twinx()
    ax2.bar(x, bundle_count, alpha=0.25, color="#ff7f0e", label="Bundle count")
    ax2.set_ylabel("Bundle count")

    lines, labels = ax1.get_legend_handles_labels()
    lines2, labels2 = ax2.get_legend_handles_labels()
    ax1.legend(lines + lines2, labels + labels2, loc="upper right")
    plt.title("Block-level Success Rate and Bundle Volume")
    fig.tight_layout()
    plt.savefig(output_dir / "block_success_rate.png", dpi=180)
    plt.close(fig)


def plot_dispatch_layers(dispatches: list[dict], output_dir: Path) -> None:
    if not dispatches:
        return
    per_block: dict[int, Counter] = defaultdict(Counter)
    for item in dispatches:
        per_block[int(item["target_block"])][item.get("layer", "unknown")] += 1

    blocks = sorted(per_block)
    exploration = [per_block[block].get("exploration", 0) for block in blocks]
    exploitation = [per_block[block].get("exploitation", 0) for block in blocks]

    plt.figure(figsize=(10, 5))
    plt.bar(blocks, exploitation, label="Exploitation")
    plt.bar(blocks, exploration, bottom=exploitation, label="Exploration")
    plt.xlabel("Target block")
    plt.ylabel("Dispatch count")
    plt.title("Dispatch Layer Mix by Block")
    plt.grid(axis="y", alpha=0.3)
    plt.legend()
    plt.tight_layout()
    plt.savefig(output_dir / "dispatch_layer_by_block.png", dpi=180)
    plt.close()


def plot_builder_scores(snapshots: list[dict], output_dir: Path) -> None:
    if not snapshots:
        return
    by_builder: dict[str, list[dict]] = defaultdict(list)
    for item in snapshots:
        by_builder[item["builder"]].append(item)

    plt.figure(figsize=(10, 5))
    for builder, rows in sorted(by_builder.items()):
        rows.sort(key=lambda item: item["recorded_at"])
        x = list(range(1, len(rows) + 1))
        y = [item.get("effective_score", 0.0) for item in rows]
        plt.plot(x, y, marker="o", label=builder)

    plt.xlabel("Observation index")
    plt.ylabel("Effective score")
    plt.title("Builder Score Trend")
    plt.grid(alpha=0.3)
    plt.legend()
    plt.tight_layout()
    plt.savefig(output_dir / "builder_score_trends.png", dpi=180)
    plt.close()


def plot_builder_dispatch_mix(dispatches: list[dict], output_dir: Path) -> None:
    if not dispatches:
        return

    counts = Counter()
    successes = Counter()
    for item in dispatches:
        builder = item["builder"]
        counts[builder] += 1
        if item.get("success"):
            successes[builder] += 1

    builders = sorted(counts)
    dispatch_count = [counts[name] for name in builders]
    success_rate = [
        successes[name] / counts[name] if counts[name] else 0.0 for name in builders
    ]

    fig, ax1 = plt.subplots(figsize=(10, 5))
    ax1.bar(builders, dispatch_count, color="#2ca02c", alpha=0.75, label="Dispatches")
    ax1.set_ylabel("Dispatch count")
    ax1.set_xlabel("Builder")
    ax1.grid(axis="y", alpha=0.3)

    ax2 = ax1.twinx()
    ax2.plot(builders, success_rate, color="#d62728", marker="o", label="Success rate")
    ax2.set_ylabel("Success rate")
    ax2.set_ylim(0, 1.05)

    lines, labels = ax1.get_legend_handles_labels()
    lines2, labels2 = ax2.get_legend_handles_labels()
    ax1.legend(lines + lines2, labels + labels2, loc="upper right")
    plt.title("Builder Dispatch Volume and Success Rate")
    fig.tight_layout()
    plt.savefig(output_dir / "builder_dispatch_mix.png", dpi=180)
    plt.close(fig)


def plot_bundle_outcomes(bundles: list[dict], output_dir: Path) -> None:
    if not bundles:
        return
    bundles = sorted(bundles, key=lambda item: item["recorded_at"])
    x = list(range(1, len(bundles) + 1))
    rolling = rolling_success_rate(
        [bool(item.get("simulation_success")) for item in bundles], window=20
    )
    profit = [wei_to_eth(item.get("profit_wei")) for item in bundles]

    fig, ax1 = plt.subplots(figsize=(10, 5))
    ax1.plot(x, rolling, color="#9467bd", label="Rolling success rate (20)")
    ax1.set_xlabel("Bundle event index")
    ax1.set_ylabel("Success rate")
    ax1.set_ylim(0, 1.05)
    ax1.grid(alpha=0.3)

    ax2 = ax1.twinx()
    ax2.plot(x, profit, color="#8c564b", alpha=0.45, label="Profit per bundle (ETH)")
    ax2.set_ylabel("ETH")

    lines, labels = ax1.get_legend_handles_labels()
    lines2, labels2 = ax2.get_legend_handles_labels()
    ax1.legend(lines + lines2, labels + labels2, loc="upper right")
    plt.title("Bundle Success and Profit Trend")
    fig.tight_layout()
    plt.savefig(output_dir / "bundle_success_profit_trend.png", dpi=180)
    plt.close(fig)


def write_summary(
    blocks: list[dict], dispatches: list[dict], snapshots: list[dict], bundles: list[dict], output_dir: Path
) -> None:
    summary = {
        "total_blocks": len(blocks),
        "total_bundle_events": len(bundles),
        "total_dispatch_events": len(dispatches),
        "total_builder_snapshots": len(snapshots),
        "exploration_dispatches": sum(
            1 for item in dispatches if item.get("layer") == "exploration"
        ),
        "exploitation_dispatches": sum(
            1 for item in dispatches if item.get("layer") == "exploitation"
        ),
        "bundle_success_rate": (
            sum(1 for item in bundles if item.get("simulation_success")) / len(bundles)
            if bundles
            else 0.0
        ),
        "total_mev_profit_eth": round(
            sum(wei_to_eth(item.get("total_mev_profit_wei")) for item in blocks), 6
        ),
    }
    with (output_dir / "summary.json").open("w", encoding="utf-8") as handle:
        json.dump(summary, handle, ensure_ascii=False, indent=2)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="绘制 rebate 实验指标图表")
    parser.add_argument(
        "--input-dir",
        default="logs/experiment",
        help="实验记录目录，默认 logs/experiment",
    )
    parser.add_argument(
        "--output-dir",
        default="logs/experiment/plots",
        help="图表输出目录，默认 logs/experiment/plots",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    input_dir = Path(args.input_dir)
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    blocks = load_jsonl(input_dir / "block_summary.jsonl")
    dispatches = load_jsonl(input_dir / "builder_dispatches.jsonl")
    snapshots = load_jsonl(input_dir / "builder_snapshots.jsonl")
    bundles = load_jsonl(input_dir / "bundle_events.jsonl")

    if not any((blocks, dispatches, snapshots, bundles)):
        raise SystemExit(f"没有在 {input_dir} 下找到可用实验数据")

    plot_block_profit(blocks, output_dir)
    plot_block_success(blocks, output_dir)
    plot_dispatch_layers(dispatches, output_dir)
    plot_builder_scores(snapshots, output_dir)
    plot_builder_dispatch_mix(dispatches, output_dir)
    plot_bundle_outcomes(bundles, output_dir)
    write_summary(blocks, dispatches, snapshots, bundles, output_dir)

    print(f"图表已生成到: {output_dir}")


if __name__ == "__main__":
    main()

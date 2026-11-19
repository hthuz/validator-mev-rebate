#!/usr/bin/env python3
"""测试mitigation服务器是否能正常启动（不实际运行）"""

import sys
from pathlib import Path

# 添加项目根目录到路径
project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

def test_server_initialization():
    """测试服务器能否正常初始化"""
    print("测试服务器初始化...\n")

    from mitigation.server import create_server
    from mitigation.strategy import ImmediateStrategy, DelayedStrategy

    # 测试创建服务器（使用立即策略）
    print("1. 创建服务器（ImmediateStrategy）...")
    server1 = create_server(strategy=ImmediateStrategy())
    print(f"   ✓ 服务器创建成功")
    print(f"   - 当前策略: {server1.strategy.get_strategy_name()}")
    print(f"   - Debug模式: {server1.debug_mode}")

    # 测试创建服务器（使用延迟策略）
    print("\n2. 创建服务器（DelayedStrategy）...")
    server2 = create_server(strategy=DelayedStrategy(delay_slots=3))
    print(f"   ✓ 服务器创建成功")
    print(f"   - 当前策略: {server2.strategy.get_strategy_name()}")

    # 测试slot manager
    print("\n3. 测试SlotManager...")
    available_slots = server1.slot_manager.get_available_slots()
    print(f"   ✓ SlotManager初始化成功")
    print(f"   - 可用slots数量: {len(available_slots)}")
    if available_slots:
        print(f"   - Slot范围: {min(available_slots)} - {max(available_slots)}")

    # 测试transaction manager
    print("\n4. 测试TransactionManager...")
    from type.models import BlockTransaction
    tx = BlockTransaction(
        slot=100,
        timestamp=0,
        signatures=["test_sig"],
        message={},
        fee=5000,
        preBalances=[],
        postBalances=[],
        instructions=[],
        logMessages=[],
    )
    server1.transaction_manager.add_transaction(tx)
    pending = server1.transaction_manager.get_pending_slots()
    print(f"   ✓ TransactionManager工作正常")
    print(f"   - 添加测试交易: {tx.signature}")
    print(f"   - 待处理slots: {pending}")

    print("\n✅ 所有服务器组件测试通过!")
    print("\n" + "="*60)
    print("服务器可以正常启动!")
    print("使用以下命令启动:")
    print("  bash mitigation/scripts/start.sh")
    print("  或")
    print("  uv run python mitigation/scripts/run.py")
    print("="*60)

if __name__ == "__main__":
    try:
        test_server_initialization()
    except Exception as e:
        print(f"\n❌ 服务器初始化测试失败: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)

"""测试mitigation导入"""
import sys
from pathlib import Path

# 添加项目根目录到路径
project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

def test_imports():
    """测试所有导入是否正常"""
    print("测试导入...")

    # 测试config
    print("✓ 导入 config")
    from config import SLOT_LEADER_PATH, DEBUG_MODE, SERVER_HOST, SERVER_PORT

    # 测试log
    print("✓ 导入 log")
    from log import server_logger

    # 测试type
    print("✓ 导入 type")
    from type.models import Transaction, SlotLeader, TransactionRecord

    # 测试mitigation模块
    print("✓ 导入 mitigation.strategy")
    from mitigation.strategy import ImmediateStrategy, DelayedStrategy

    print("✓ 导入 mitigation.slot_manager")
    from mitigation.slot_manager import SlotManager

    print("✓ 导入 mitigation.transaction_manager")
    from mitigation.transaction_manager import TransactionManager

    print("✓ 导入 mitigation.server")
    from mitigation.server import TransactionServer, create_server

    print("\n✅ 所有导入测试通过!")
    return True

if __name__ == "__main__":
    try:
        test_imports()
    except Exception as e:
        print(f"\n❌ 导入测试失败: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)

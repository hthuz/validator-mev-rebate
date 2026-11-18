"""示例：如何使用mitigation客户端发送交易"""

import sys
from pathlib import Path
import requests
import time

# 添加项目根目录到路径
project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

from config import SERVER_HOST, SERVER_PORT

BASE_URL = f"http://{SERVER_HOST}:{SERVER_PORT}"

def check_health():
    """检查服务器健康状态"""
    response = requests.get(f"{BASE_URL}/health")
    return response.json()

def send_transaction(tx_id: str, slot: int, signature: str, data: dict = None):
    """发送交易到服务器"""
    payload = {
        "tx_id": tx_id,
        "slot": slot,
        "signature": signature,
        "data": data or {}
    }
    response = requests.post(f"{BASE_URL}/sendTransaction", json=payload)
    return response.json()

def get_pending_transactions():
    """获取待处理交易"""
    response = requests.get(f"{BASE_URL}/transactions/pending")
    return response.json()

def get_sent_transactions():
    """获取已发送交易"""
    response = requests.get(f"{BASE_URL}/transactions/sent")
    return response.json()

def get_current_slot():
    """获取当前slot"""
    response = requests.get(f"{BASE_URL}/slot/current")
    return response.json()

def advance_slot():
    """手动推进slot（仅debug模式）"""
    response = requests.post(f"{BASE_URL}/slot/advance")
    return response.json()

def example_usage():
    """示例使用流程"""
    print("Mitigation客户端示例\n" + "="*60)

    # 1. 检查服务器状态
    print("\n1. 检查服务器健康状态...")
    health = check_health()
    print(f"   状态: {health['status']}")
    print(f"   当前slot: {health['current_slot']}")
    print(f"   策略: {health['strategy']}")
    print(f"   Debug模式: {health['debug_mode']}")

    # 2. 发送几笔交易
    print("\n2. 发送交易...")
    current_slot = health['current_slot']

    tx1 = send_transaction(
        tx_id="tx_001",
        slot=current_slot + 5,
        signature="sig_001",
        data={"amount": 1000, "token": "SOL"}
    )
    print(f"   ✓ 交易1: {tx1}")

    tx2 = send_transaction(
        tx_id="tx_002",
        slot=current_slot + 10,
        signature="sig_002",
        data={"amount": 2000, "token": "USDC"}
    )
    print(f"   ✓ 交易2: {tx2}")

    # 3. 查看待处理交易
    print("\n3. 查看待处理交易...")
    pending = get_pending_transactions()
    print(f"   待处理slots: {pending['pending_slots']}")
    print(f"   数量: {pending['count']}")

    # 4. 如果是debug模式，演示手动推进slot
    if health['debug_mode']:
        print("\n4. 手动推进slot（Debug模式）...")
        print("   按Enter键推进slot，或使用 POST /slot/advance")
        print("   交易会根据策略在合适的时机发送")
    else:
        print("\n4. 自动模式下，slot会自动推进")
        print("   交易会根据策略在合适的时机发送")

    print("\n" + "="*60)
    print("完成! 使用以下API与服务器交互:")
    print("  GET  /health - 健康检查")
    print("  POST /sendTransaction - 发送交易")
    print("  GET  /transactions/pending - 待处理交易")
    print("  GET  /transactions/sent - 已发送交易")
    print("  GET  /slot/current - 当前slot")
    print("  POST /slot/advance - 推进slot（debug模式）")

if __name__ == "__main__":
    try:
        example_usage()
    except requests.exceptions.ConnectionError:
        print("\n❌ 无法连接到服务器")
        print("请先启动服务器:")
        print("  bash mitigation/scripts/start.sh")
    except Exception as e:
        print(f"\n❌ 错误: {e}")
        import traceback
        traceback.print_exc()

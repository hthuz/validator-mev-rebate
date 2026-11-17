
import matplotlib.pyplot as plt
import seaborn as sns
import numpy as np

# visualize a list, for test purpose
def visualize_list(data: list):
    plt.plot(data, marker='o')
    plt.title("List Visualization - Line Plot")
    plt.xlabel("Index")
    plt.ylabel("Value")
    plt.grid(True)
    plt.show()


    
def moving_average(data, window_size=3):
    return np.convolve(data, np.ones(window_size)/window_size, mode='same')

def visualize_epoch(data: dict):
    
    epochs = sorted(data.keys())
    data_values = [data[e] for e in epochs]

    data_values = moving_average(data_values, window_size=3)
    plt.figure(figsize=(14, 6))

    plt.plot(epochs, data_values, label='value', color='tab:blue', linewidth=2)

    # 加入稀疏的标记点（每10个标一次）
    interval = max(1, len(epochs) // 10)
    plt.scatter(epochs[::interval], data_values[::interval], color='tab:blue', marker='o', s=30, label='_nolegend_')

    # for i in range(0, len(epochs), interval):
    #     plt.text(epochs[i], data_values[i], f"{data_values[i]}", fontsize=10, ha='center', va='bottom')

    # 设置图形属性
    plt.title("epoch dynamics", fontsize=18)
    plt.xlabel("Epoch", fontsize=14)
    plt.ylabel("Value", fontsize=14)
    plt.legend()
    plt.grid(True, linestyle='--', alpha=0.5)

    # 限制X轴刻度显示数量，避免重叠
    plt.xticks(epochs[::interval], rotation=45)

    plt.tight_layout()
    plt.show()
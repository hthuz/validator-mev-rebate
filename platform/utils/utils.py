
import os
import numpy as np

import matplotlib.pyplot as plt

def update_plt():
    plt.rcParams['font.family'] = 'serif'
    # plt.rcParams['font.serif'] = ['Times New Roman', 'Times', 'DejaVu Serif']
    # plt.rcParams['font.sans-serif'] = ['Arial', 'Helvetica', 'DejaVu Sans']
    # plt.rcParams['font.monospace'] = ['Courier New', 'Courier', 'Consolas']
    return


def find_match_path(auction_path: str, epoch: int) -> str:
    dirs = os.listdir(auction_path)
    epoch_path = ""
    for dir in dirs:
        if dir.split('.')[0] == str(epoch):
            epoch_path = os.path.join(auction_path, dir)
    return epoch_path
    


def tosol(amount: int) -> float:
    return amount / 1e9

def abbreviate_addr(addr: str) -> str:
    return addr[:4] + ".." + addr[-4:]
    


# for drawing

def human_format_lamport(x, pos):
    return human_format_sol(tosol(x), pos)

def human_format_sol(x, pos):
    if x >= 1_000_000_000_000:
            return f'{x/1_000_000_000_000:.1f}T'
    elif x >= 1_000_000_000:
        return f'{x/1_000_000_000:.1f}B'
    elif x >= 1_000_000:
        return f'{x/1_000_000:.1f}M'
    elif x >= 1_000:
        return f'{x/1_000:.1f}K'
    else:

        return f'{x:.0f}'
def format_sol(n: float) -> str:
    """
    将大数字转为可读形式，例如：
    1011 -> '1k'
    300213 -> '300k'
    1331431 -> '1.33M'
    """
    abs_n = abs(n)
    if abs_n < 1_000:
        return str(n)
    elif abs_n < 1_000_000:
        return f'{n/1_000:.0f} k'
    elif abs_n < 1_000_000_000:
        return f'{n/1_000_000:.1f} M'
    elif abs_n < 1_000_000_000_000:
        return f'{n/1_000_000_000:.1f} B'
    else:
        return f'{n/1_000_000_000_000:.1f} T'

def format_lamport(n: int) -> str:
    return format_sol(tosol(n))


def print_sorted(dic: dict):
    sorted_dict = {key: dic[key] for key in sorted(dic)}
    print(sorted_dict)

    
# def gini(x):
#     # if len(x) == 0:
#     #     return np.nan
#     total = 0
#     for i, xi in enumerate(x[:-1], 1):
#         total += np.sum(np.abs(xi - x[i:]))
#     return total / (len(x)**2 * np.mean(x))



# def gini(x: list | np.ndarray) -> float:
#     x = np.array(x, dtype=np.float64)
#     if np.any(x < 0):
#         raise ValueError("All values must be non-negative.")
#     if np.all(x == 0):
#         return 0.0

#     x_sorted = np.sort(x)
#     n = len(x)
#     cumx = np.cumsum(x_sorted)
#     gini = (2 * np.sum((np.arange(1, n+1)) * x_sorted)) / (n * np.sum(x_sorted)) - (n + 1) / n
#     return gini

def gini(array: list):
    """Calculate the Gini coefficient of a numpy array."""
    # based on bottom eq:
    # http://www.statsdirect.com/help/generatedimages/equations/equation154.svg
    # from:
    # http://www.statsdirect.com/help/default.htm#nonparametric_methods/gini.htm
    # All values are treated equally, arrays must be 1d:
    array = np.array(array)
    
    array = array.flatten()
    # Values cannot be 0:
    array = array + 0.0000001
    # Values must be sorted:
    array = np.sort(array)
    # Index per array element:
    index = np.arange(1,array.shape[0]+1)
    # Number of array elements:
    n = array.shape[0]
    # Gini coefficient:
    return ((np.sum((2 * index - n  - 1) * array)) / (n * np.sum(array)))

def HHI(x: list):
    """
    Compute the Herfindahl-Hirschman Index (HHI).
    """
    total = sum(x)
    proportions = np.array([v / total for v in x])
    
    # HHI
    hhi = np.sum(proportions ** 2)
    # Calculate HHI as the sum of squared proportions
    
    # Scale HHI to a 0-10,000 range
    return hhi

def nakamoto_coefficient(stakes: list):
    threshold = 0.33
    stakes = sorted(stakes, reverse=True)
    total_stake = sum(stakes)
    target_stake = threshold * total_stake

    running_sum = 0
    for i, v in enumerate(stakes):
        running_sum += v
        if running_sum >= target_stake:
            return i + 1
    return len(stakes)
    


def num_switch(locs: list[str]) -> int:
    if len(locs) <= 1:
        return 0
    # return sum(c1 != c2 for c1, c2 in zip(locs, locs[1:]))
    return sum(locs[i] != locs[i+1] for i in range(0, len(locs)-1))
    
def geostability(locs: list[str]) -> float:
    if len(locs) <= 1:
        return 1
    switches = num_switch(locs)
    return 1 - switches / (len(locs) - 1)

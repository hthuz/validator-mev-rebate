
import pandas as pd
import seaborn as sns
import numpy as np
import matplotlib.pyplot as plt


def correlation_analysis(df: pd.DataFrame, corr_cols: list = None, drop_cols: list = None, save: bool=False, figname: str=None):
    if corr_cols == None:
        if drop_cols != None:
            df = df.drop(columns=drop_cols)
        corr_cols = list[df.columns]
    corr_matrix = df[corr_cols].corr()
    print(corr_matrix)

    mask = np.eye(len(corr_matrix), dtype=bool)
    

    plt.figure(figsize=(10, 8))
    sns.heatmap(
        corr_matrix, 
        annot=True, 
        cmap='coolwarm', 
        fmt=".2f",            
        linewidths=0.5,       
        linecolor='white',    
        square=True,          
        cbar_kws={'shrink': 0.8},
        mask=mask,
    )
    plt.xticks(rotation=45, ha='right', fontsize=14)
    plt.yticks(rotation=0, fontsize=14)
    plt.tight_layout()
    # plt.show()



    if save:
        if not figname:
            plt.savefig("correlation_map.png")
        else:
            plt.savefig(f"{figname}.pdf")
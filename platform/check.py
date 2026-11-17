

import reader
from config import *
if __name__ == "__main__":


    for epoch in range (START_EPOCH, END_EPOCH):
        if epoch in EXCLUDE_EPOCHS:
            continue
        print(f"=== Epoch {epoch} ===", flush=True)
        r = reader.Reader(epoch)
        r.check()

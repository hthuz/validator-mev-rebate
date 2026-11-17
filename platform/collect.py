

import os
import json
import subprocess
import time
import requests
from concurrent.futures import ThreadPoolExecutor, as_completed
from config import *


MaxRetry = 10

def log(msg):
    print(msg, flush=True)

def safe_subprocess_run(cmd, retries=10, delay=2):
    for attempt in range(retries):
        try:
            result = subprocess.run(cmd, capture_output=True, text=True)
            if result.returncode == 0:
                return result
            else:
                log(f"Command failed (attempt {attempt+1}): {' '.join(cmd)}\n{result.stderr.strip()}")
        except Exception as e:
            log(f"Subprocess error: {e}")
        time.sleep(delay)
    return None


class Collector:
    def __init__(self):
        self.epoch: int = 0
        self.epoch_path = os.path.join(EPOCHS_PATH, str(self.epoch))
        self.max_workers = COLLECT_WORKER
        self.validators_list: list[dict]
    def update_epoch(self):
        if self.epoch == 0:
            result = safe_subprocess_run([SOLANA_BIN, "epoch"], 10)
            if not result:
                raise RuntimeError("failed to retrive epoch info")
            self.epoch = int(result.stdout.rstrip())

        self.epoch_path = os.path.join(EPOCHS_PATH, str(self.epoch))
        os.makedirs(self.epoch_path, exist_ok=True)
        log(f"Epoch {self.epoch}")
    
    def update_validators(self):
        path = os.path.join(self.epoch_path, "validators.json")
        if os.path.exists(path):
            with open(path, 'r') as f:
                validators = json.load(f)
                self.validators_list = validators["validators"]
                self.validators_list.sort(key=lambda x: x["voteAccountPubkey"])
                log(f"Loaded {len(self.validators_list)} validators from file")
            return

        result = safe_subprocess_run([SOLANA_BIN, "validators", "--keep-unstaked-delinquents", "--output", "json"])
        if not result:
            log("Failed to retrive validator list")
            return

        result = result.stdout.rstrip()
        with open(path, "w") as f:
            f.write(result)
        validators = json.loads(result)
        self.validators_list = validators["validators"]
        self.validators_list.sort(key=lambda x: x["voteAccountPubkey"])
        log(f"Fetched and saved {len(self.validators_list)} validators")

    def update_validator_info(self):
        path = os.path.join(self.epoch_path, "validator_info.json")
        validator_list = []
        if os.path.exists(path):
            with open(path, 'r') as f:
                validator_list = json.load(f)
                log(f"Loaded {len(validator_list)} validators with info from file")
            return

        result = safe_subprocess_run([SOLANA_BIN, "validator-info", "get", "--output", "json"])
        if not result:
            log("Failed to retrieve validator-info.")
            return

        result = result.stdout.rstrip()
        with open(path, "w") as f:
            f.write(result)
        validator_list = json.loads(result)
        log(f"Fetch and saved {len(validator_list)} validator info")

    def update_stake(self):
        with ThreadPoolExecutor(max_workers=self.max_workers) as executor:
            futures = [
                executor.submit(self.process_stake, validator) 
                for validator in self.validators_list
            ]
            for future in as_completed(futures):
                future.result()
        
        
    # an individual task to collect stake
    def process_stake(self, validator: dict, retry=0):
        vote_account = validator["voteAccountPubkey"]
        validator_path = os.path.join(self.epoch_path, f"{vote_account}.json")
        error_validator_path = os.path.join(self.epoch_path, f"ERROR_{vote_account}.json")
        if os.path.exists(validator_path) or os.path.exists(error_validator_path):
            return

        print(f"Getting stakers for {vote_account}", flush=True)
        result = subprocess.run([
            SOLANA_BIN,
            "stakes",
            vote_account,
            "--output", "json"
            ], capture_output=True, text=True)
        error = result.stderr.rstrip()
        if error:
            print(f"Getting stake err {error}", flush=True)
            if error.startswith("Error: RPC request error: Failed to retrieve matching vote account for"):
                with open(error_validator_path, 'w') as f:
                    f.write(error)
            else:
                if retry >= MaxRetry:
                    with open("retry_failed.txt", 'w') as f:
                        f.write(vote_account, error, "\n")
                else:
                    time.sleep(1)
                    self.process_stake(validator, retry+1)
                return
        else:
            with open(validator_path, 'w') as f:
                f.write(result.stdout.rstrip())
    
    def stakewiz(self):
        url = "https://api.stakewiz.com/validators"
        retries = 5
        output_file = os.path.join(self.epoch_path, "stakewiz.json")
        for attempt in range(retries):
            try:
                response = requests.get(url, timeout=10)
                response.raise_for_status()  # Raises HTTPError for bad responses (4xx or 5xx)
                data = response.json()

                with open(output_file, "w", encoding="utf-8") as f:
                    json.dump(data, f, indent=2, ensure_ascii=False)

                print(f"Stakewiz data successfully saved to {output_file}", flush=True)
                return True
            except requests.RequestException as e:
                print(f"Stakewiz attempt {attempt + 1} failed: {e}", flush=True)
                if attempt < retries - 1:
                    time.sleep(2 ** attempt)
                else:
                    print("Stakewiz all attempts failed.", flush=True)
                    return False
        return

    def run(self):
        self.update_epoch()
        self.stakewiz()
        self.update_validators()
        self.update_validator_info()
        self.update_stake()
    

if __name__ == "__main__":
    Collector().run()



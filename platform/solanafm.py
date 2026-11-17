import requests
import json

from time import sleep
from reader import Reader


epoch = 800
r = Reader(epoch)
stakers = r.get_stakers().keys()
print(f"Total stakers: {len(stakers)}")

url = 'https://api.solana.fm/v0/accounts'
headers = {
    'accept': 'application/json',
    'content-type': 'application/json'
}
batch_size = 30000
all_results = []

for i in range(0, len(stakers), batch_size):
    batch = stakers[i:i + batch_size]
    data = {
        'accountHashes': batch
    }
    try:
        response = requests.post(url, headers=headers, data=json.dumps(data))
        response.raise_for_status()
        result = response.json()
        batch_result = result.get("result", [])
        all_results.extend(batch_result)
        print(f"Processed batch {i // batch_size + 1} with {len(batch)} accounts")
        sleep(1)
    except requests.exceptions.RequestException as err:
        print(f"Error processing batch {i // batch_size + 1}: {err}")

with open("solanafm.json", 'w') as f:
    json.dump(all_results, f)

print("All batches processed and saved.")

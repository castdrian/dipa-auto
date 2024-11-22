import argparse
import requests
import json
import time
import os
import logging
import hashlib
from datetime import datetime
from pathlib import Path

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)

class DipaChecker:
    def __init__(self, mock_hash=None, repo_name=None):
        self.base_url = "https://ipa.aspy.dev/discord"
        self.github_token = os.getenv("REPO_PAT")
        self.hash_file = Path("/var/lib/dipa-auto/branch_hashes.json")
        self.hash_file.parent.mkdir(parents=True, exist_ok=True)
        self.mock_hash = mock_hash
        self.repo_name = repo_name or "bunny-mod/BunnyTweak"
        self.load_hashes()

    def load_hashes(self):
        if self.hash_file.exists():
            with open(self.hash_file) as f:
                self.branch_hashes = json.load(f)
                if self.mock_hash:
                    self.branch_hashes["stable"] = self.mock_hash
        else:
            self.branch_hashes = {"stable": None, "testflight": None}

    def fetch_ipa_list(self, branch):
        response = requests.get(
            f"{self.base_url}/{branch}/",
            headers={"Accept": "application/json"}
        )
        data = response.json()
        
        if self.mock_hash and branch == "stable":
            return data, self.mock_hash
        
        return data, hashlib.sha256(json.dumps(data, sort_keys=True).encode()).hexdigest()

    def get_latest_version(self, ipa_list):
        if not ipa_list:
            return None
        return max(ipa_list, key=lambda x: x["mod_time"])

    def dispatch_github_workflow(self, ipa_url, is_testflight):
        if not self.github_token:
            raise Exception("GitHub token not found")
        
        logging.info(f"Dispatching workflow for {ipa_url}")
        response = requests.post(
            f"https://api.github.com/repos/{self.repo_name}/dispatches",
            headers={
                "Accept": "application/vnd.github+json",
                "Authorization": f"Bearer {self.github_token}",
                "X-GitHub-Api-Version": "2022-11-28"
            },
            json={
                "event_type": "ipa-update",
                "client_payload": {
                    "ipa_url": ipa_url,
                    "is_testflight": is_testflight
                }
            }
        )
        return response.status_code == 204

    def check_branch(self, branch):
        logging.info(f"Checking {branch} branch...")
        try:
            ipa_list, current_hash = self.fetch_ipa_list(branch)
            
            if current_hash != self.branch_hashes[branch]:
                latest_version = self.get_latest_version(ipa_list)
                if latest_version:
                    final_url = f"{self.base_url}/{branch}/{latest_version['name']}"
                    logging.info(f"New version found in {branch}: {final_url}")
                    
                    if self.dispatch_github_workflow(final_url, branch == "testflight"):
                        logging.info("GitHub workflow dispatched successfully")
                        self.branch_hashes[branch] = current_hash
                        self.save_hashes()
                    else:
                        raise Exception("GitHub workflow dispatch failed")
            else:
                logging.info(f"No changes detected in {branch}")

        except Exception as e:
            logging.error(f"Error checking {branch} branch: {str(e)}")
            return False
        return True

    def save_hashes(self):
        with open(self.hash_file, "w") as f:
            json.dump(self.branch_hashes, f)

    def run(self):
        while True:
            current_time = datetime.now()
            seconds_until_next_hour = 3600 - (current_time.minute * 60 + current_time.second)
            
            self.check_branch("stable")
            self.check_branch("testflight")
            
            time.sleep(seconds_until_next_hour)

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--mock-hash", help="Mock hash for testing")
    args = parser.parse_args()
    
    checker = DipaChecker(args.mock_hash)
    checker.run() 
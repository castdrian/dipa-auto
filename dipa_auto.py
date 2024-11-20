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
    def __init__(self, mock_hash=None):
        self.base_url = "https://ipa.aspy.dev/discord"
        self.github_token = os.getenv("REPO_PAT")
        self.hash_file = Path("/var/lib/dipa-auto/branch_hashes.json")
        self.hash_file.parent.mkdir(parents=True, exist_ok=True)
        self.mock_hash = mock_hash
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
            "https://api.github.com/repos/castdrian/apt-repo/dispatches",
            headers={
                "Accept": "application/vnd.github.v3+json",
                "Authorization": f"token {self.github_token}"
            },
            json={
                "event_type": "ipa-update",
                "client_payload": {
                    "ipa_url": ipa_url,
                    "is_testflight": str(is_testflight).lower()
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

    def run(self):
        while True:
            if not self.check_branch("stable"):
                time.sleep(3600)
                continue
            
            time.sleep(300)
            
            if not self.check_branch("testflight"):
                time.sleep(3300)
                continue
                
            time.sleep(3300)

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--mock-hash", help="Mock hash for testing")
    args = parser.parse_args()
    
    checker = DipaChecker(args.mock_hash)
    checker.run() 
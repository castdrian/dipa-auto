import argparse
import requests
import json
import time
import os
import logging
import hashlib
import tomli
import zon
from datetime import datetime
from pathlib import Path
from croniter import croniter

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)

# Try to use array if available, fall back to list, or use a custom validator
try:
    CONFIG_SCHEMA = zon.record({
        "ipa_base_url": zon.string().url(),
        "refresh_schedule": zon.string().regex(r"^(\d+,)*\d+\s+(\d+,)*\d+|\*\s+(\d+,)*\d+|\*\s+(\d+,)*\d+|\*\s+(\d+,)*\d+|\*$"),
        "targets": zon.array(zon.record({
            "github_token": zon.string().regex(r"^(gh[ps]_[a-zA-Z0-9]{36}|github_pat_[a-zA-Z0-9]{22}_[a-zA-Z0-9]{59})$"),
            "github_repo": zon.string().regex(r"^[a-zA-Z0-9-]+/[a-zA-Z0-9-]+$")
        })).min(1)
    })
except AttributeError:
    try:
        CONFIG_SCHEMA = zon.record({
            "ipa_base_url": zon.string().url(),
            "refresh_schedule": zon.string().regex(r"^(\d+,)*\d+\s+(\d+,)*\d+|\*\s+(\d+,)*\d+|\*\s+(\d+,)*\d+|\*\s+(\d+,)*\d+|\*$"),
            "targets": zon.list(zon.record({
                "github_token": zon.string().regex(r"^(gh[ps]_[a-zA-Z0-9]{36}|github_pat_[a-zA-Z0-9]{22}_[a-zA-Z0-9]{59})$"),
                "github_repo": zon.string().regex(r"^[a-zA-Z0-9-]+/[a-zA-Z0-9-]+$")
            })).min(1)
        })
    except AttributeError:
        # If neither array nor list methods are available, use a custom validator
        class ConfigValidator:
            def validate(self, data):
                if not isinstance(data.get("ipa_base_url"), str):
                    raise ValueError("ipa_base_url must be a string URL")
                if not isinstance(data.get("refresh_schedule"), str):
                    raise ValueError("refresh_schedule must be a cron expression string")
                if not isinstance(data.get("targets"), list) or len(data["targets"]) < 1:
                    raise ValueError("targets must be an array with at least one item")
                    
                # Validate cron expression
                try:
                    croniter(data["refresh_schedule"], datetime.now())
                except Exception as e:
                    raise ValueError(f"Invalid cron expression: {e}")
                    
                for target in data["targets"]:
                    if not isinstance(target.get("github_repo"), str):
                        raise ValueError("Each target must have a github_repo string")
                    if not isinstance(target.get("github_token"), str):
                        raise ValueError("Each target must have a github_token string")
                        
                return data
        
        CONFIG_SCHEMA = ConfigValidator()

class DipaChecker:
    def __init__(self, mock_hash=None):
        self.config_path = os.getenv("CONFIG_PATH", "config.toml")
        self.load_config()
        
        self.hash_file = Path("/var/lib/dipa-auto/branch_hashes.json")
        self.hash_file.parent.mkdir(parents=True, exist_ok=True)
        self.mock_hash = mock_hash
        self.load_hashes()

    def load_config(self):
        try:
            with open(self.config_path, "rb") as f:
                config = tomli.load(f)
            
            # Validate config using zon
            validated_config = CONFIG_SCHEMA.validate(config)
            
            self.base_url = validated_config["ipa_base_url"]
            self.refresh_schedule = validated_config["refresh_schedule"]
            self.targets = validated_config["targets"]
            
        except zon.error.ZonError as e:
            logging.error(f"Config validation failed: {e}")
            raise
        except Exception as e:
            logging.error(f"Error loading config: {e}")
            raise

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
        success = True
        for target in self.targets:
            logging.info(f"Dispatching workflow for {ipa_url} to {target['github_repo']}")
            try:
                response = requests.post(
                    f"https://api.github.com/repos/{target['github_repo']}/dispatches",
                    headers={
                        "Accept": "application/vnd.github+json",
                        "Authorization": f"Bearer {target['github_token']}",
                    },
                    json={
                        "event_type": "ipa-update",
                        "client_payload": {
                            "ipa_url": ipa_url,
                            "is_testflight": is_testflight
                        }
                    }
                )
                if response.status_code != 204:
                    logging.error(f"Failed to dispatch workflow to {target['github_repo']}")
                    success = False
            except Exception as e:
                logging.error(f"Error dispatching to {target['github_repo']}: {str(e)}")
                success = False
        return success

    def check_branch(self, branch):
        logging.info(f"Checking {branch} branch...")
        try:
            ipa_list, current_hash = self.fetch_ipa_list(branch)
            
            if current_hash != self.branch_hashes[branch]:
                latest_version = self.get_latest_version(ipa_list)
                if (latest_version):
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
            self.check_branch("stable")
            self.check_branch("testflight")
            
            # Calculate sleep time until next run based on cron schedule
            now = datetime.now()
            cron = croniter(self.refresh_schedule, now)
            next_run = cron.get_next(datetime)
            sleep_seconds = (next_run - now).total_seconds()
            
            logging.info(f"Next check scheduled at {next_run.strftime('%Y-%m-%d %H:%M:%S')} (in {sleep_seconds:.0f} seconds)")
            time.sleep(sleep_seconds)

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--mock-hash", help="Mock hash for testing")
    args = parser.parse_args()
    
    checker = DipaChecker(args.mock_hash)
    checker.run()

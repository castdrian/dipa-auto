import json
import os
from unittest.mock import patch, MagicMock
from dipa_auto import DipaChecker
import logging

def test_new_version_detection():
    mock_response = [
        {
            "name": "Discord_255.0.ipa",
            "size": 99742540,
            "url": "./Discord_255.0.ipa",
            "mod_time": "2024-11-19T05:12:40.190413201Z",
            "mode": 420,
            "is_dir": False,
            "is_symlink": False
        }
    ]

    with patch('requests.get') as mock_get:
        mock_get.return_value.json.return_value = mock_response
        
        with patch('requests.post') as mock_post:
            mock_post.return_value.status_code = 204
            
            checker = DipaChecker(
                mock_hash="different_hash_to_trigger_update",
                repo_name="castdrian/PyoncordTweak"
            )
            
            checker.branch_hashes = {
                "stable": "old_hash_stable",
                "testflight": "old_hash_testflight"
            }
            
            print("Testing stable branch...")
            assert checker.check_branch("stable"), "Stable branch check failed"
            mock_post.assert_called_with(
                "https://api.github.com/repos/castdrian/PyoncordTweak/dispatches",
                headers={
                    "Accept": "application/vnd.github+json",
                    "Authorization": f"Bearer {checker.github_token}",
                    "X-GitHub-Api-Version": "2022-11-28"
                },
                json={
                    "event_type": "ipa-update",
                    "client_payload": {
                        "ipa_url": "https://ipa.aspy.dev/discord/stable/Discord_255.0.ipa",
                        "is_testflight": "false"
                    }
                }
            )
            
            print("Testing testflight branch...")
            mock_post.reset_mock()
            assert checker.check_branch("testflight"), "Testflight branch check failed"
            assert mock_post.called, "GitHub workflow was not dispatched for testflight"
            
            print("✅ All tests passed successfully")

if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    test_new_version_detection() 
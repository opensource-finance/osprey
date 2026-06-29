Output demos/evaluate.gif
Output demos/evaluate.mp4

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 40ms

# Title
Type "# Osprey — Real-time Transaction Monitoring"
Enter
Type "# Single binary. 12 FATF rules. < 5ms evaluation."
Enter
Enter
Sleep 2s

# 1. Normal transaction
Type "# 1) Normal $250 transfer"
Enter
Sleep 500ms
Type `curl -s http://localhost:9090/evaluate \`
Enter
Type `  -H "Content-Type: application/json" \`
Enter
Type `  -H "X-Tenant-ID: kutanapay" \`
Enter
Type `  -d '{"type":"TRANSFER","debtor":{"id":"user-001","accountId":"acc-001"},"creditor":{"id":"user-002","accountId":"acc-002"},"amount":{"value":250,"currency":"USD"}}' | jq .`
Enter
Sleep 3s

Enter
Type "# Result: NALT (no alert) — score 0.05"
Enter
Sleep 2s

Enter
# 2. Account drain
Type "# 2) Suspicious: $15K account drain to zero"
Enter
Sleep 500ms
Type `curl -s http://localhost:9090/evaluate \`
Enter
Type `  -H "Content-Type: application/json" \`
Enter
Type `  -H "X-Tenant-ID: kutanapay" \`
Enter
Type `  -d '{"type":"CASH_OUT","debtor":{"id":"user-003","accountId":"acc-003"},"creditor":{"id":"user-004","accountId":"acc-004"},"amount":{"value":15000,"currency":"USD"},"metadata":{"old_balance":15000,"new_balance":0}}' | jq .`
Enter
Sleep 3s

Enter
Type "# Result: ALRT — 9 reasons triggered, score 0.42"
Enter
Sleep 2s

Enter
# 3. Self-transfer (layering)
Type "# 3) Same-party transfer (layering pattern)"
Enter
Sleep 500ms
Type `curl -s http://localhost:9090/evaluate \`
Enter
Type `  -H "Content-Type: application/json" \`
Enter
Type `  -H "X-Tenant-ID: kutanapay" \`
Enter
Type `  -d '{"type":"TRANSFER","debtor":{"id":"user-005","accountId":"acc-005"},"creditor":{"id":"user-005","accountId":"acc-006"},"amount":{"value":9800,"currency":"USD"}}' | jq .`
Enter
Sleep 3s

Enter
Type "# Result: ALRT — same party + structuring range"
Enter
Sleep 3s

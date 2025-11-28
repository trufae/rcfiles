# Notes

## Admin Ollama

```bash
systemctl edit ollama.service
systemctl daemon-reload
systemctl restart ollama
```

https://atlassc.net/2025/01/15/configuring-your-ollama-server

ollama list | awk 'NR>1 {print $1}' | xargs -I {} sh -c 'echo "Updating model: {}"; ollama pull {}; echo "-"' && echo "All models updated."


## clean caches

```bash
echo "disable vm/nr_hugepage"
echo 0 | tee /proc/sys/vm/nr_hugepages

echo "Starting cache cleaner - Running"
echo "Press Ctrl + C to stop"
while true; do
	sync && echo 3 | tee /proc/sys/vm/drop_caches > /dev/null
	sleep 3
done
```

## Sniffing

sudo tcpdump -n -A "tcp port 11434"

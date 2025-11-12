sudo tee /etc/keyd/default.conf <<'EOF'
[ids]
*

[main]
rightshift = 102nd
S-rightshift = S-102nd
EOF
sudo systemctl restart keyd


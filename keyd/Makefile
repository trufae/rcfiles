all:
	git clone https://github.com/rvaiya/keyd
	cd keyd && make && sudo make install
	sh keyd.sh
	systemctl restart udevmon

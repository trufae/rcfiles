all: dwm
	cd dwm && git reset --hard && git clean -xdf && git pull
	cd dwm && patch -p1 < ../dwm-fibonacci-6.2.diff
	cp config.h dwm/config.h
	make -C dwm
	sudo rm -f /usr/bin/dwm
	sudo cp dwm/dwm /usr/bin/dwm

dwm:
	git clone git://git.suckless.org/dwm

clean:
	rm -rf dwm

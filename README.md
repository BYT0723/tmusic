# TMusic

A Go-based music tool

## Origin

本人正在使用Linux, Linux本机与QQMusic的联动可以说是非常烂，让我不得不使用MPD，但是我长期在移动端使用QQMusic, 我的许多歌单和歌曲都在QQMusic的云端，
故此写此工具用于将歌单同步到本地, 此前也用NodeJS实现过一版[music-downloader](https://github.com/BYT0723/music-downloader)，但是依赖NodeJS。

## Usage

### 同步QQMusic自建歌单到本地

使用时需要QQMusic Cookie, 你可把cookie写入`./qqmusic-cookie.txt`或者`-c <cookie_file>`去指定cookie文件

默认歌曲存放到`$HOME/Music`, 歌词则`$HOME/Music/.lyrics`

```shell
tmusic qqmusic sync -c <cookie_file>

# 过滤正常日志输出
tmusic qqmusic sync -c <cookie_file> -s

# 指定目录
tmusic qqmusic sync -c <cookie_file> --song-dir $HOME/Music --lyric-dir $HOME/Music/.lyrics

# 生成MPD Playlist
tmusic qqmusic sync -c <cookie_file> -g

# 指定Playlist Path
tmusic qqmusic sync -c <cookie_file> -g --mpd-playlist <path>
```

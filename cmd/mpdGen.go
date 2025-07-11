/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/BYT0723/tmusic/utils/log"
	"github.com/spf13/cobra"
)

// mpdGenCmd represents the mpdGen command
var mpdGenCmd = &cobra.Command{
	Use:   "mpd-gen",
	Short: "generate mpd playlist from music directory",
	Long:  `generate mpd playlist from music directory`,
	Run: func(cmd *cobra.Command, args []string) {
		generateMpdPlaylists()
	},
}

var mpdMusicDir, mpdPlaylistDir string

func init() {
	rootCmd.AddCommand(mpdGenCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// mpdGenCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// mpdGenCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	home, _ := os.UserHomeDir()

	mpdGenCmd.Flags().StringVarP(&mpdMusicDir, "music-dir", "m", filepath.Join(home, "Music"), "music directory")
	mpdGenCmd.Flags().StringVarP(&mpdPlaylistDir, "playlist-dir", "p", filepath.Join(home, ".mpd", "playlists"), "playlist directory")
}

var mpdSupportExt = []string{".mp3", ".flac", ".m4a"}

func generateMpdPlaylists() {
	if _, err := os.Stat(mpdMusicDir); os.IsNotExist(err) {
		log.Errorf("music directory %s 不存在\n", mpdMusicDir)
		os.Exit(1)
	}

	_ = os.MkdirAll(mpdPlaylistDir, os.ModePerm)

	de, err := os.ReadDir(mpdMusicDir)
	if err != nil {
		log.Errorf("music directory %s 读取失败: %v\n", mpdMusicDir, err)
		os.Exit(1)
	}

	for _, d := range de {
		if d.IsDir() {
			p := filepath.Join(mpdMusicDir, d.Name())
			items, err := os.ReadDir(p)
			if err != nil {
				log.Errorf("music directory %s 读取失败: %v\n", p, err)
				continue
			}
			var buf bytes.Buffer
			for _, item := range items {
				if item.IsDir() || !slices.Contains(mpdSupportExt, filepath.Ext(item.Name())) {
					continue
				}
				buf.WriteString(filepath.Join(d.Name(), item.Name()))
				buf.WriteString("\n")
			}
			if buf.Len() > 0 {
				if err := os.WriteFile(filepath.Join(mpdPlaylistDir, fmt.Sprintf("%s.m3u", d.Name())), buf.Bytes(), os.ModePerm); err != nil {
					log.Errorf("写入 %s.m3u 失败: %v\n", d.Name(), err)
				}
			}
		}
	}
}

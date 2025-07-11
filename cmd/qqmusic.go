/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

var cookiePath string

// qqmusicCmd represents the qqmusic command
var qqmusicCmd = &cobra.Command{
	Use:   "qqmusic",
	Short: "toolset for qqmusic",
	Long:  `toolset for qqmusic`,
	Run: func(cmd *cobra.Command, args []string) {
	},
}

func init() {
	rootCmd.AddCommand(qqmusicCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// qqmusicCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// qqmusicCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	qqmusicCmd.PersistentFlags().StringVarP(&cookiePath, "cookie", "c", "./qqmusic-cookie.txt", "cookie filepath")
}

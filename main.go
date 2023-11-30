package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"

	"github.com/giulianopz/go-gsst/pkg/client"
	"github.com/giulianopz/go-gsst/pkg/logger"
	"github.com/giulianopz/go-gsst/pkg/opts"
	"github.com/giulianopz/go-gsst/pkg/str"
	goflac "github.com/go-flac/go-flac"
)

const usage = `Usage:
    gstt [OPTION]... -key $KEY -output [pb|json]
    gstt [OPTION]... -key $KEY --interim -continuous -output [pb|json]

Options:
	--verbose
	--file, path of audio file to trascript
	--key, api key built into chromium
	--output, transcriptions output format ('pb' for binary or 'json' for text)
	--language, language of the recording transcription, use the standard webcodes for your language, i.e. 'en-US' for English-US, 'ru' for Russian, etc. please, see https://en.wikipedia.org/wiki/IETF_language_tag
	--continuous, to keep the stream open and transcoding as long as there is no silence
	--interim, to send back results before its finished, so you get a live stream of possible transcriptions as it processes the audio
	--max-alts, how many possible transcriptions do you want
	--pfilter, profanity filter ('0'=off, '1'=medium, '2'=strict)
	--user-agent, user-agent for spoofing
`

var (
	verbose    bool
	filePath   string
	apiKey     string
	output     string
	language   string
	continuous bool
	interim    bool
	maxAlts    string
	pFilter    string
	userAgent  string
)

func main() {

	flag.BoolVar(&verbose, "verbose", false, "verbose")
	flag.StringVar(&filePath, "file", "", "path of audio file to trascript")
	flag.StringVar(&apiKey, "key", "", "API key built into Chrome")
	flag.StringVar(&output, "output", "", "output format ('pb' for binary or 'json' for text)")
	flag.StringVar(&language, "language", "null", "language of the recording transcription, use the standard codes for your language, i.e. 'en-US' for English-US, 'ru' for Russian, etc. please, see https://en.wikipedia.org/wiki/IETF_language_tag")
	flag.BoolVar(&continuous, "continuous", false, "to keep the stream open and transcoding as long as there is no silence")
	flag.BoolVar(&interim, "interim", false, "to send back results before its finished, so you get a live stream of possible transcriptions as it processes the audio")
	flag.StringVar(&maxAlts, "max-alts", "1", "how many possible transcriptions do you want")
	flag.StringVar(&pFilter, "pfilter", "2", "profanity filter ('0'=off, '1'=medium, '2'=strict)")
	flag.StringVar(&userAgent, "user-agent", opts.DefaultUserAgent, "user-agent for spoofing (default 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36')")
	flag.Usage = func() { fmt.Print(usage) }
	flag.Parse()

	if verbose {
		logger.Level(slog.LevelDebug)
	}

	var (
		c       = client.New()
		options = fromFlags()
	)

	if filePath != "" { // transcribe from file

		f, err := goflac.ParseFile(filePath)
		if err != nil {
			logger.Error("cannot parse file", "err", err)
			os.Exit(1)
		}
		data, err := f.GetStreamInfo()
		if err != nil {
			logger.Error("cannot get file info", "err", err)
			os.Exit(1)
		}
		logger.Info("done parsing file", "sample rate", data.SampleRate)

		c.Stream(bytes.NewBuffer(f.Marshal()), data.SampleRate, options)

	} else { // transcribe from microphone input

		// 1kB chunk size
		bs := make([]byte, 1024)

		// stream POST request body with a pipe
		pr, pw := io.Pipe()
		go func() {
			defer pr.Close()
			defer pw.Close()

			c.Stream(pr, opts.DefaultSampleRate, options)
		}()

		for {
			n, err := os.Stdin.Read(bs)
			if n > 0 {
				logger.Debug("read from stdin", "bs", bs)

				_, err := pw.Write(bs)
				if err != nil {
					panic(err)
				}
			} else if err == io.EOF {
				logger.Info("done reading from stdin")
				break
			} else if err != nil {
				logger.Error("cannot not read from stdin", "err", err)
				os.Exit(1)
			}
		}
	}
}

func fromFlags() *opts.Options {

	options := make([]opts.Option, 0)

	if verbose {
		options = append(options, opts.Verbose(true))
	}
	if filePath != "" {
		options = append(options, opts.FilePath(filePath))
	}
	if apiKey != "" {
		options = append(options, opts.ApiKey(apiKey))
	}
	if output != "" {
		if output == "json" {
			options = append(options, opts.Output(opts.Text))
		} else {
			options = append(options, opts.Output(opts.Binary))
		}
	}
	if language != "" {
		options = append(options, opts.Language(language))
	}
	if continuous {
		options = append(options, opts.Continuous(true))
	}
	if interim {
		options = append(options, opts.Interim(true))
	}
	if maxAlts != "" {
		num, err := strconv.Atoi(maxAlts)
		if err != nil {
			panic(err)
		}
		options = append(options, opts.MaxAlts(num))
	}
	if pFilter != "" {
		num, err := strconv.Atoi(pFilter)
		if err != nil {
			panic(err)
		}
		options = append(options, opts.ProfanityFilter(num))
	}
	options = append(options, opts.UserAgent(str.GetOrDefault(userAgent, opts.DefaultUserAgent)))

	return opts.Apply(options...)
}

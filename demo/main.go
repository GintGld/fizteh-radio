package main

import (
	"context"
	"demo/stream"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	bcg, err := stream.Init(time.Now(), time.Second, time.Second)
	if err != nil {
		panic(err)
	}

	bcg.Manifest.Path = "tmp/test.mpd"

	bcg.Store = stream.NewMixer(
		"tmp/source-samples/Green Apelsin - Проклятие русалки (mp3cut.net).mp3",
		"tmp/source-samples/Hozier - Angel Of Small Death & The Codeine Scene (mp3cut.net).mp3",
		"tmp/source-samples/The Killers - Mr. Brightside (mp3cut.net).mp3",
		"tmp/source-samples/Ляпис Трубецкой - В платье белом (mp3cut.net).mp3",
		"tmp/source-samples/Порнофильмы - Приезжай! (mp3cut.net).mp3",
	)

	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)

	go bcg.Run(ctx)

	fs := http.FileServer(http.Dir("tmp"))
	srv := &http.Server{Addr: ":3000", Handler: fs}

	// srv.Handle("/", fs)
	go srv.ListenAndServe()

	// http.HandleFunc("/what", func(w http.ResponseWriter, r *http.Request) {
	// 	fmt.Fprint(w, "Hey")
	// })

	fmt.Println("Server is listening...")
	// go http.ListenAndServe(":3000", nil)

	<-ctx.Done()

	if err := srv.Shutdown(ctx); err != nil {
		panic(err)
	}

	// if err := bcg.Run(ctx); err != nil {
	// 	fmt.Println(err)
	// }
}

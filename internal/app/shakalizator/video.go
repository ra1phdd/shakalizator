package shakalizator

import (
	"fmt"
	"math"
	"shakalizator/pkg/ffmpeg"
	"strconv"
	"strings"
)

func ShakalizeVideo(inputPath, outputPath string, level int) error {
	ffprobe, err := ffmpeg.NewFfprobe()
	if err != nil {
		return err
	}
	out, err := ffprobe.Verbose("quiet").
		OutputFormat("json").
		ShowStreams().
		ShowEntries("").
		Execute(inputPath)
	if err != nil {
		return err
	}

	scale, videoBitrate, audioBitrate, fps := calculateShakalizeParams(&out, level)
	fmt.Println(scale, videoBitrate, audioBitrate, fps)

	//args := []string{
	//	"-i", inputPath,
	//	"-vf", fmt.Sprintf("scale=%s:flags=neighbor, scale=iw*2:ih*2:flags=neighbor", scale),
	//	"-c:v", "libx264",
	//	"-b:v", videoBitrate,
	//	"-r", fmt.Sprintf("%d", fps),
	//	"-y", outputPath,
	//}
	//
	//if metadata.AudioBitrate > 0 {
	//	args = append(args,
	//		"-c:a", "aac",
	//		"-b:a", audioBitrate,
	//		"-ar", "22050",
	//		"-ac", "1",
	//	)
	//}
	//
	//cmd := exec.Command("ffmpeg", args...)
	//if err := cmd.Run(); err != nil {
	//	log.Printf("FFmpeg error: %v", err)
	//	return err
	//}
	return nil
}

func calculateShakalizeParams(out *ffmpeg.FFProbeOutput, level int) (string, string, string, int) {
	levelScale := 1.1 - float64(level)/10.0

	newWidth := int(float64(out.Streams[0].Width) * levelScale)
	newHeight := int(float64(out.Streams[0].Height) * levelScale)

	newWidth = newWidth - (newWidth % 2)
	newHeight = newHeight - (newHeight % 2)

	videoBr, err := strconv.ParseFloat(out.Streams[0].BitRate, 64)
	if err != nil {
		fmt.Println("Ошибка преобразования:", err)
	}

	audioBr, err := strconv.ParseFloat(out.Streams[1].BitRate, 64)
	if err != nil {
		fmt.Println("Ошибка преобразования:", err)
	}

	videoBitrate := videoBr * levelScale
	audioBitrate := audioBr * levelScale

	fps, err := convertFps(out.Streams[0].AvgFrameRate)
	if err != nil {
		return "", "", "", 0
	}
	fps *= levelScale

	return fmt.Sprintf("%d:%d", newWidth, newHeight),
		fmt.Sprintf("%.0fk", videoBitrate),
		fmt.Sprintf("%.0fk", audioBitrate),
		int(fps)
}

func convertFps(avgFrameRate string) (float64, error) {
	fpsParts := strings.Split(avgFrameRate, "/")
	if len(fpsParts) != 2 {
		return 0, fmt.Errorf("invalid avg frame rate: %s", avgFrameRate)
	}

	one, err := strconv.ParseFloat(fpsParts[0], 64)
	if err != nil {
		return 0, err
	}

	two, err := strconv.ParseFloat(fpsParts[1], 64)
	if err != nil {
		return 0, err
	}

	fps := one
	if two != 0 {
		fps = math.Round(one/two*100) / 100
	}

	return fps, nil
}

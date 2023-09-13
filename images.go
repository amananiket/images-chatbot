package main

import (
    "fmt"
    "log"
    "os"
    "strings"
    "path/filepath"

    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/cors"
    "github.com/google/uuid"
    "net/http"
    "io"
    "strconv"
    "sync"
)

type image struct {
	tags []string
	imageUrl string
	imageData map[string]interface{}
}

type classScore struct {
	score float64
	idx int
}

var currentTask string

var images []image

var tasks = [...]string{"save image", "search image", "refresh context"}

var taskPrompts = [...]string{
"Sure. Please upload the image you want to save and add tags",
 "Sounds good. Please share your search tags separated by commas",
 "Your context is refreshed. You are good to go. "}


func main() {
	
	service := fiber.New()
	service.Use(cors.New())

    service.Static("/images", "./images")

    
	service.Post("/", handleFileUpload)
	
	service.Post("/callback", handleCallback)
    
    log.Fatal(service.Listen(":4000"))

}

func getSimilarityScore(class string, i int, text string, c chan classScore, wg *sync.WaitGroup) {
	defer wg.Done()
	url := "http://localhost:1234/word2vec/n_similarity?"
	log.Println("class : ", class)
	log.Println("text : ", text)
	
	classWords := strings.Split(class, " ")
	textWords := strings.Split(text, " ")
	
	for i := range classWords {
		url += "ws1=" + classWords[i] + "&"
	}
	
	for i := range textWords {
		url += "ws2=" + textWords[i] + "&"
	}
	
	resp, _ := http.Get(url)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	score, _ := strconv.ParseFloat(strings.TrimSuffix(string(body), "\n"), 64)
	log.Println("score : ", score, " i : ", i)
	c <- classScore{score : score, idx : i}
}

func getTagsSimilarity(imageTags []string, i int, inputTags []string, c chan classScore, wg *sync.WaitGroup) {
	defer wg.Done()
	url := "http://localhost:1234/word2vec/n_similarity?"
	
	for i := range imageTags {
		url += "ws1=" + imageTags[i] + "&"
	}
	
	for i := range inputTags {
		url += "ws2=" + inputTags[i] + "&"
	}
	
	resp, _ := http.Get(url)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	score, _ := strconv.ParseFloat(strings.TrimSuffix(string(body), "\n"), 64)
	log.Println("score : ", score, " i : ", i)
	c <- classScore{score : score, idx : i}
}

func RemoveContents(dir string) error {
    d, err := os.Open(dir)
    if err != nil {
        return err
    }
    defer d.Close()
    names, err := d.Readdirnames(-1)
    if err != nil {
        return err
    }
    for _, name := range names {
        err = os.RemoveAll(filepath.Join(dir, name))
        if err != nil {
            return err
        }
    }
    return nil
}


func handleCallback(c *fiber.Ctx) error {
	if currentTask != "" {
		if (currentTask == tasks[0]) {
			handleFileUpload(c)
		} else if (currentTask == tasks[1]) {
			handleFileSearch(c)
		}
	} else {
		// Set current task 
		log.Println(string(c.Body()))
		ch := make(chan classScore, len(tasks))
		var wg sync.WaitGroup
		
		for i := range tasks {
			wg.Add(1)
			go getSimilarityScore(tasks[i], i, string(c.Body()), ch, &wg)
		}
		
		maxScore := classScore{}
		wg.Wait()
		close(ch)
		log.Println("wait done")
		maxScore = <- ch
		
		for i := range ch {
			log.Println(i)
			if i.score > maxScore.score {
				//log.Println(i)
				maxScore = i
			}
		
			
		}
		
		currentTask = tasks[maxScore.idx]
		
		if (maxScore.idx == 2) {
			currentTask = ""
			refreshContext()
		}
		
		log.Println(maxScore)
		
		return c.JSON(taskPrompts[maxScore.idx])
		
	}
	return nil
}

func refreshContext() {
	RemoveContents("./images/")
	images = nil
}

func handleFileSearch(c *fiber.Ctx) error {
	inputTags := c.FormValue("tags")
	inputTagsArray := strings.Split(inputTags, ",")
	
	ch := make(chan classScore, len(images))
	var wg sync.WaitGroup
	
	log.Println("images = ", len(images))
	for i := range images {
		wg.Add(1)
		go getTagsSimilarity(images[i].tags, i, inputTagsArray, ch, &wg)
	}
	
	maxScore := classScore{}
	wg.Wait()
	close(ch)
	log.Println("wait done")
	maxScore = <- ch
	
	for i := range ch {
		log.Println(i)
		if i.score > maxScore.score {
			//log.Println(i)
			maxScore = i
		}
	
		
	}
	
	log.Println(maxScore)
	currentTask = ""
	
	log.Println(images[maxScore.idx])
	return c.JSON("This is the image you were searching for : " + images[maxScore.idx].imageUrl)
	
}


func handleFileUpload(c *fiber.Ctx) error {

    // parse incomming image file

    file, err := c.FormFile("image")

    if err != nil {
        log.Println("image upload error --> ", err)
        return c.JSON(fiber.Map{"status": 500, "message": "Server error", "data": nil})

    }
    
    inputTags := c.FormValue("tags")

    // generate new uuid for image name 
    uniqueId := uuid.New()

    // remove "- from imageName"

    filename := strings.Replace(uniqueId.String(), "-", "", -1)

    // extract image extension from original file filename

    fileExt := strings.Split(file.Filename, ".")[1]

    // generate image from filename and extension
    imageFile := fmt.Sprintf("%s.%s", filename, fileExt)

    // save image to ./images dir 
    err = c.SaveFile(file, fmt.Sprintf("./images/%s", imageFile))

    if err != nil {
        log.Println("image save error --> ", err)
        return c.JSON(fiber.Map{"status": 500, "message": "Server error", "data": nil})
    }

    // generate image url to serve to client using CDN

    imageUrl := fmt.Sprintf("http://localhost:4000/images/%s", imageFile)

    // create meta data and send to client

    data := map[string]interface{}{

        "imageName": imageFile,
        "imageUrl":  imageUrl,
        "header":    file.Header,
        "size":      file.Size,
    }
    
    uploadedImage := image{tags : strings.Split(inputTags, ","), imageUrl : imageUrl, imageData : data}
    
    images = append(images, uploadedImage)
    
    log.Println(images)
    currentTask = ""

    return c.JSON("Your image was successfully uploaded. What do you want to do next?")
}

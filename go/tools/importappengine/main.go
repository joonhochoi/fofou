package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var dataDir = ""
var APP_NAME = "SumatraPDF"

// data dir is ../../../data on the server or ../../fofoudata locally
// the important part is that it's outside of the code
func getDataDir() string {
	if dataDir != "" {
		return dataDir
	}
	dataDir = filepath.Join("..", "..", "fofoudata")
	if PathExists(dataDir) {
		return dataDir
	}
	dataDir = filepath.Join("..", "..", "..", "data")
	if PathExists(dataDir) {
		return dataDir
	}
	log.Fatal("data directory (../../../data or ../../fofoudata) doesn't exist")
	return ""
}

func dataFilePath(app string) string {
	return filepath.Join(getDataDir(), app, "data.txt")
}

type Post struct {
	ForumId      int
	TopicId      int
	CreatedOn    string
	MessageSha1  [20]byte
	IsDeleted    bool
	IP           string
	UserName     string
	UserEmail    string
	UserHomepage string

	Id int
}

type Topic struct {
	ForumId   int
	Id        int
	Subject   string
	CreatedOn string
	CreatedBy string
	IsDeleted bool
	Posts     []*Post
}

var newlines = []byte{'\n', '\n'}
var newline = []byte{'\n'}

func parseTopic(d []byte) *Topic {
	parts := bytes.Split(d, newline)
	topic := &Topic{}
	for _, p := range parts {
		lp := bytes.Split(p, []byte{':', ' '})
		name := string(lp[0])
		val := string(lp[1])
		if "I" == name {
			idparts := strings.Split(val, ".")
			topic.ForumId, _ = strconv.Atoi(idparts[0])
			topic.Id, _ = strconv.Atoi(idparts[1])
		} else if "S" == name {
			topic.Subject = val
		} else if "On" == name {
			// TODO: change to time.Time
			topic.CreatedOn = val
		} else if "By" == name {
			topic.CreatedBy = val
		} else if "D" == name {
			topic.IsDeleted = ("True" == val)
		} else {
			log.Fatalf("Unknown topic name: %s\n", name)
		}
	}
	return topic
}

func parsePost(d []byte) *Post {
	parts := bytes.Split(d, newline)
	post := &Post{}
	for _, p := range parts {
		lp := bytes.Split(p, []byte{':', ' '})
		name := string(lp[0])
		val := string(lp[1])
		if "T" == name {
			idparts := strings.Split(val, ".")
			post.ForumId, _ = strconv.Atoi(idparts[0])
			post.TopicId, _ = strconv.Atoi(idparts[1])
		} else if "On" == name {
			// TODO: change to time.Time
			post.CreatedOn = val
		} else if "M" == name {
			sha1, err := hex.DecodeString(val)
			if err != nil || len(sha1) != 20 {
				log.Fatalf("error decoding M")
			}
			copy(post.MessageSha1[:], sha1)
		} else if "D" == name {
			post.IsDeleted = ("True" == val)
		} else if "IP" == name {
			post.IP = val
		} else if "UN" == name {
			post.UserName = val
		} else if "UE" == name {
			post.UserEmail = val
		} else if "UH" == name {
			post.UserHomepage = val
		} else {
			log.Fatalf("Unknown post name: %s\n", name)
		}
	}
	return post
}

func parseTopics(d []byte) []*Topic {
	topics := make([]*Topic, 0)
	for len(d) > 0 {
		idx := bytes.Index(d, newlines)
		if idx == -1 {
			break
		}
		topic := parseTopic(d[:idx])
		topics = append(topics, topic)
		d = d[idx+2:]
	}
	return topics
}

func loadTopics() []*Topic {
	data_dir := filepath.Join("..", "appengine", "imported_data")
	file_path := filepath.Join(data_dir, "topics.txt")
	f, err := os.Open(file_path)
	if err != nil {
		log.Fatalf("failed to open %s with error %s", file_path, err.Error())
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatalf("ReadAll() failed with error %s", err.Error())
	}
	return parseTopics(data)
}

func parsePosts(d []byte) []*Post {
	posts := make([]*Post, 0)
	for len(d) > 0 {
		idx := bytes.Index(d, newlines)
		if idx == -1 {
			break
		}
		post := parsePost(d[:idx])
		posts = append(posts, post)
		d = d[idx+2:]
	}
	return posts
}

func loadPosts() []*Post {
	data_dir := filepath.Join("..", "appengine", "imported_data")
	file_path := filepath.Join(data_dir, "posts.txt")
	f, err := os.Open(file_path)
	if err != nil {
		log.Fatalf("failed to open %s with error %s", file_path, err.Error())
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatalf("ReadAll() failed with error %s", err.Error())
	}
	return parsePosts(data)
}

func orderTopicsAndPosts(topics []*Topic, posts []*Post) []*Topic {
	res := make([]*Topic, 0)
	idToTopic := make(map[int]*Topic)
	deletedTopics := make(map[int]*Topic)
	droppedTopics := 0

	for _, t := range topics {
		if t.ForumId != 1 || t.IsDeleted {
			droppedTopics += 1
			deletedTopics[t.Id] = t
			continue
		}
		idToTopic[t.Id] = t
		res = append(res, t)
	}

	droppedPosts := 0
	nPosts := 0
	for _, p := range posts {
		if p.ForumId != 1 {
			droppedPosts += 1
			continue
		}

		t, ok := idToTopic[p.TopicId]
		if !ok {
			if _, ok = deletedTopics[p.TopicId]; !ok {
				panic("didn't find topic")
			}
			droppedPosts += 1
			continue
		}
		if p.IsDeleted {
			droppedPosts += 1
			continue
		}

		if nil == t.Posts {
			t.Posts = make([]*Post, 0)
			/*
				if t.CreatedBy != p.UserName {
					fmt.Printf("%v\n", t)
					fmt.Printf("%v\n", p)
					log.Fatalf("Mismatched names: t.CreatedBy=%s != p.UserName=%s", t.CreatedBy, p.UserName)
				}
				if t.CreatedOn != p.CreatedOn {
					log.Fatalf("Mismtached times: t.CreatedOn=%s != p.CreatedOn=%s", t.CreatedOn, p.CreatedOn)
				}*/
		}
		t.Posts = append(t.Posts, p)
		nPosts += 1
	}

	// TODO: need to order t.Posts in each post by time, I think

	// renumber ids sequentially for compactness
	tId := 1
	pId := 1
	for _, t := range res {
		t.Id = tId
		for _, p := range t.Posts {
			p.TopicId = tId
			p.Id = pId
			pId += 1
		}
		tId += 1
	}
	fmt.Printf("Dropped topics: %d, dropped posts: %d, total posts: %d\n", droppedTopics, droppedPosts, nPosts)
	return res
}

var sep = "|"

func remSep(s string) string {
	return strings.Replace(s, sep, "", -1)
}

func serTopic(t *Topic) string {
	if t.IsDeleted {
		panic("t.IsDeleted is true")
	}
	return fmt.Sprintf("T:%d|%s\n", t.Id, remSep(t.Subject))
}

var b64encoder = base64.StdEncoding

func serPost(p *Post) string {
	if p.IsDeleted {
		panic("p.IsDeleted is true")
	}
	s1 := remSep(p.CreatedOn)
	s2 := b64encoder.EncodeToString(p.MessageSha1[:])
	s3 := remSep(p.UserName)
	s4 := remSep(p.UserEmail)
	s5 := remSep(p.UserHomepage)
	return fmt.Sprintf("P:%d|%d|%s|%s|%s|%s|%s|%s\n", p.TopicId, p.Id, s1, s2, p.IP, s3, s4, s5)
}

func serializePostsAndTopics(topics []*Topic) string {
	s := ""
	for _, t := range topics {
		s += serTopic(t)
		for _, p := range t.Posts {
			s += serPost(p)
		}
	}
	return s
}

func main() {
	topics := loadTopics()
	posts := loadPosts()
	topics = orderTopicsAndPosts(topics, posts)
	s := serializePostsAndTopics(topics)

	f, err := os.Create(dataFilePath(APP_NAME))
	if err != nil {
		log.Fatalf("os.Create() failed with %s", err.Error())
	}
	defer f.Close()
	_, err = f.WriteString(s)
	if err != nil {
		log.Fatalf("WriteFile() failed with %s", err.Error())
	}
}
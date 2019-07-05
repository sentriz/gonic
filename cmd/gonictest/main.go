package main

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/sentriz/gonic/scanner/tags"
)

var tracks = []string{
	"/home/senan/music/Jah Wobble, The Edge, Holger Czukay/(1983) Snake Charmer/01.05 Snake Charmer.flac",
	"/home/senan/music/Jah Wobble, The Edge, Holger Czukay/(1983) Snake Charmer/03.05 It Was a Camel.flac",
	"/home/senan/music/Jah Wobble, The Edge, Holger Czukay/(1983) Snake Charmer/02.05 Hold On to Your Dreams.flac",
	"/home/senan/music/Jah Wobble, The Edge, Holger Czukay/(1983) Snake Charmer/05.05 Snake Charmer (reprise).flac",
	"/home/senan/music/Jah Wobble, The Edge, Holger Czukay/(1983) Snake Charmer/04.05 Sleazy.flac",
	"/home/senan/music/A Certain Ratio/(1994) The Graveyard and the Ballroom/13.14 Flight.flac",
	"/home/senan/music/A Certain Ratio/(1994) The Graveyard and the Ballroom/05.14 Flight.flac",
	"/home/senan/music/A Certain Ratio/(1994) The Graveyard and the Ballroom/14.14 Genotype_Phenotype.flac",
	"/home/senan/music/A Certain Ratio/(1994) The Graveyard and the Ballroom/09.14 Oceans.flac",
	"/home/senan/music/A Certain Ratio/(1994) The Graveyard and the Ballroom/08.14 All Night Party.flac",
	"/home/senan/music/A Certain Ratio/(1994) The Graveyard and the Ballroom/03.14 Crippled Child.flac",
	"/home/senan/music/A Certain Ratio/(1994) The Graveyard and the Ballroom/12.14 Suspect.flac",
	"/home/senan/music/A Certain Ratio/(1994) The Graveyard and the Ballroom/02.14 Faceless.flac",
	"/home/senan/music/A Certain Ratio/(1994) The Graveyard and the Ballroom/11.14 The Fox.flac",
	"/home/senan/music/A Certain Ratio/(1994) The Graveyard and the Ballroom/10.14 The Choir.flac",
	"/home/senan/music/A Certain Ratio/(1994) The Graveyard and the Ballroom/04.14 Choir.flac",
	"/home/senan/music/A Certain Ratio/(1994) The Graveyard and the Ballroom/01.14 Do the Du (casse).flac",
	"/home/senan/music/A Certain Ratio/(1994) The Graveyard and the Ballroom/06.14 I Feel.flac",
	"/home/senan/music/A Certain Ratio/(1994) The Graveyard and the Ballroom/07.14 Strain.flac",
	"/home/senan/music/A Certain Ratio/(1981) To EachOTHER./07.09 Loss.flac",
	"/home/senan/music/A Certain Ratio/(1981) To EachOTHER./01.09 Felch.flac",
	"/home/senan/music/A Certain Ratio/(1981) To EachOTHER./05.09 Back to the Start.flac",
	"/home/senan/music/A Certain Ratio/(1981) To EachOTHER./08.09 Oceans.flac",
	"/home/senan/music/A Certain Ratio/(1981) To EachOTHER./06.09 The Fox.flac",
	"/home/senan/music/A Certain Ratio/(1981) To EachOTHER./02.09 My Spirit.flac",
	"/home/senan/music/A Certain Ratio/(1981) To EachOTHER./04.09 Choir.flac",
	"/home/senan/music/A Certain Ratio/(1981) To EachOTHER./09.09 Winter Hill.flac",
	"/home/senan/music/A Certain Ratio/(1981) To EachOTHER./03.09 Forced Laugh.flac",
	"/home/senan/music/13th Floor Lowervators/(1967) Easter Nowhere/07.10 Dust.flac",
	"/home/senan/music/13th Floor Lowervators/(1967) Easter Nowhere/01.10 Slip Inside This House.flac",
	"/home/senan/music/13th Floor Lowervators/(1967) Easter Nowhere/08.10 Levitation.flac",
	"/home/senan/music/13th Floor Lowervators/(1967) Easter Nowhere/02.10 Slide Machine.flac",
	"/home/senan/music/13th Floor Lowervators/(1967) Easter Nowhere/03.10 She Lives (In a Time of Her Own).flac",
	"/home/senan/music/13th Floor Lowervators/(1967) Easter Nowhere/10.10 Pictures (Leave Your Body Behind).flac",
	"/home/senan/music/13th Floor Lowervators/(1967) Easter Nowhere/04.10 Nobody to Love.flac",
	"/home/senan/music/13th Floor Lowervators/(1967) Easter Nowhere/05.10 Baby Blue.flac",
	"/home/senan/music/13th Floor Lowervators/(1967) Easter Nowhere/06.10 Earthquake.flac",
	"/home/senan/music/13th Floor Lowervators/(1967) Easter Nowhere/09.10 I Had to Tell You.flac",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/19.21 She Lives (In a Time of Her Own).mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/13.21 Before You Accuse Me.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/18.21 Gloria.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/12.21 Everybody Needs Someone to Love.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/02.21 Roller Coaster.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/03.21 Splash 1.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/05.21 Don't Fall Down.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/16.21 Roll Over Beethoven.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/08.21 You Don't Know.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/07.21 Thru the Rhythm.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/20.21 We Sell Soul.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/10.21 Monkey Island.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/21.21 You're Gonna Miss Me.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/17.21 The Word.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/04.21 Reverberation (Doubt).mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/06.21 Fire Engine.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/01.21 You're Gonna Miss Me.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/09.21 Kingdom of Heaven.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/14.21 I'm Gonna Love You Too.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/11.21 Tried to Hide.mp3",
	"/home/senan/music/13th Floor Lowervators/(1966) The Psychedelic Sounds of the 13th Floor Elevators/15.21 You Really Got Me.mp3",
	"/home/senan/music/___Anika/Hello/There/(2010) Anika/06.09 Sadness Hides the Sun.flac",
	"/home/senan/music/___Anika/Hello/There/(2010) Anika/08.09 I Go to Sleep.flac",
	"/home/senan/music/___Anika/Hello/There/(2010) Anika/05.09 Officer Officer.flac",
	"/home/senan/music/___Anika/Hello/There/(2010) Anika/09.09 Masters of War (dub).flac",
	"/home/senan/music/___Anika/Hello/There/(2010) Anika/07.09 No One's There.flac",
	"/home/senan/music/___Anika/Hello/There/(2010) Anika/01.09 Terry.flac",
	"/home/senan/music/___Anika/Hello/There/(2010) Anika/04.09 Masters of War.flac",
	"/home/senan/music/___Anika/Hello/There/(2010) Anika/03.09 End of the World.flac",
	"/home/senan/music/___Anika/Hello/There/(2010) Anika/02.09 Yang Yang.flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/01.16 Robot Factory.flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/04.16 Cake Shop Girl.flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/10.16 ...vs. the Mangrove Delta Plan.flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/05.16 The Helicopter Spies.flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/12.16 Whatever Happens Next..flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/08.16 Mining Villages.flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/14.16 A Raincoat's Room.flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/13.16 Blenheim Shots.flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/15.16 The Stairs Are Like an Avalanche.flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/06.16 Big Maz in the Desert.flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/11.16 Secret Island.flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/02.16 Let's Buy a Bridge.flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/09.16 Collision With a Frogman..flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/07.16 Big Empty Field.flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/03.16 Border Country.flac",
	"/home/senan/music/Swell Maps/(1980) Jane From Occupied Europe/16.16 New York.flac",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d02 02.04 Doctor at Cake.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d01 06.14 Don't Throw Ashtrays at Me!.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d01 02.14 Another Song.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d01 11.14 Full Moon (reprise).mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d01 04.14 Spitfire Parade.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d01 09.14 Full Moon in My Pocket.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d01 05.14 Harmony in Your Bathroom.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d01 12.14 Gunboats.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d01 13.14 Adventuring Into Basketry.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d01 08.14 Bridge Head, Part 9.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d02 03.04 Steven Does.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d01 07.14 Midget Submarines.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d02 01.04 Loin of the Surf.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d01 14.14 My Lil' Shoppes 'Round the Corner.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d01 10.14 BLAM!!.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d01 03.14 Vertical Slum.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d01 01.14 H.S. Art.mp3",
	"/home/senan/music/Swell Maps/(1979) A Trip to Marineville/d02 04.04 Bronze & Baby Shoes.mp3",
	"/home/senan/music/Ten Years After/(1967) Ten Years After/02.15 I Can't Keep From Crying, Sometimes.ogg",
	"/home/senan/music/Ten Years After/(1967) Ten Years After/15.15 Woodchopper's Ball.ogg",
	"/home/senan/music/Ten Years After/(1967) Ten Years After/12.15 Rock Your Mama.ogg",
	"/home/senan/music/Ten Years After/(1967) Ten Years After/06.15 Feel It for Me.ogg",
	"/home/senan/music/Ten Years After/(1967) Ten Years After/13.15 Spider in My Web.ogg",
	"/home/senan/music/Ten Years After/(1967) Ten Years After/04.15 Spoonful.ogg",
	"/home/senan/music/Ten Years After/(1967) Ten Years After/05.15 Losing the Dogs.ogg",
	"/home/senan/music/Ten Years After/(1967) Ten Years After/09.15 Help Me.ogg",
	"/home/senan/music/Ten Years After/(1967) Ten Years After/03.15 Adventures of a Young Organ.ogg",
	"/home/senan/music/Ten Years After/(1967) Ten Years After/01.15 I Want to Know.ogg",
	"/home/senan/music/Ten Years After/(1967) Ten Years After/10.15 Portable People.ogg",
	"/home/senan/music/Ten Years After/(1967) Ten Years After/11.15 The Sounds.ogg",
	"/home/senan/music/Ten Years After/(1967) Ten Years After/14.15 Hold Me Tight.ogg",
	"/home/senan/music/Ten Years After/(1967) Ten Years After/07.15 Love Until I Die.ogg",
	"/home/senan/music/Ten Years After/(1967) Ten Years After/08.15 Don't Want You Woman.ogg",
	"/home/senan/music/Captain Beefheart/(1970) Lick My Decals Off, Bitch/15.15 Flash Gordon's Ape.mp3",
	"/home/senan/music/Captain Beefheart/(1970) Lick My Decals Off, Bitch/01.15 Lick My Decals Off, Baby.mp3",
	"/home/senan/music/Captain Beefheart/(1970) Lick My Decals Off, Bitch/05.15 Bellerin' Plain.mp3",
	"/home/senan/music/Captain Beefheart/(1970) Lick My Decals Off, Bitch/10.15 One Red Rose That I Mean.mp3",
	"/home/senan/music/Captain Beefheart/(1970) Lick My Decals Off, Bitch/06.15 Woe-Is-Uh-Me-Bop.mp3",
	"/home/senan/music/Captain Beefheart/(1970) Lick My Decals Off, Bitch/07.15 Japan in a Dishpan.mp3",
	"/home/senan/music/Captain Beefheart/(1970) Lick My Decals Off, Bitch/11.15 The Buggy Boogie Woogie.mp3",
	"/home/senan/music/Captain Beefheart/(1970) Lick My Decals Off, Bitch/12.15 The Smithsonian Institute Blues (Or the Big Dig).mp3",
	"/home/senan/music/Captain Beefheart/(1970) Lick My Decals Off, Bitch/14.15 The Clouds Are Full of Wine (Not Whiskey or Rye).mp3",
	"/home/senan/music/Captain Beefheart/(1970) Lick My Decals Off, Bitch/04.15 Peon.mp3",
	"/home/senan/music/Captain Beefheart/(1970) Lick My Decals Off, Bitch/08.15 I Wanna Find a Woman That'll Hold My Big Toe Till I Have to Go.mp3",
	"/home/senan/music/Captain Beefheart/(1970) Lick My Decals Off, Bitch/13.15 Space-Age Couple.mp3",
	"/home/senan/music/Captain Beefheart/(1970) Lick My Decals Off, Bitch/09.15 Petrified Forest.mp3",
	"/home/senan/music/Captain Beefheart/(1970) Lick My Decals Off, Bitch/02.15 Doctor Dark.mp3",
	"/home/senan/music/Captain Beefheart/(1970) Lick My Decals Off, Bitch/03.15 I Love You, You Big Dummy.mp3",
}

func worker(in <-chan string, out chan<- *tags.Tags) {
	for {
		path, ok := <-in
		if !ok {
			return
		}
		trtags, err := tags.New(path)
		if err != nil {
			log.Fatalf("err: %v\n", err)
		}
		out <- trtags
	}
}

func main() {
	runtime.GOMAXPROCS(16)
	ins := make(chan string)
	outs := make(chan *tags.Tags)
	times := 600
	go func() {
		for i := 0; i < times; i++ {
			for _, path := range tracks {
				ins <- path
			}
		}
		close(ins)
	}()
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			worker(ins, outs)
			wg.Done()
		}()
		fmt.Println("started worker")
	}
	start := time.Now()
	var res int
	go func() {
		for _ = range outs {
			res++
		}
	}()
	wg.Wait()
	fmt.Println("got", len(tracks)*times, "were", res)
	log.Printf("took %v\n", time.Since(start))
}

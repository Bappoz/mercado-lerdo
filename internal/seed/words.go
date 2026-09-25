package seed

type category struct {
	name  string
	nouns []string
}

var categories = []category{
	{"Home", []string{"Lamp", "Chair", "Shelf", "Rug", "Mirror", "Clock", "Vase", "Cushion", "Blanket", "Stool", "Planter", "Candle"}},
	{"Kitchen", []string{"Kettle", "Pan", "Knife", "Mug", "Bowl", "Grinder", "Board", "Teapot", "Skillet", "Jar", "Whisk", "Tray"}},
	{"Office", []string{"Desk", "Notebook", "Pen", "Stapler", "Organizer", "Monitor Stand", "Folder", "Lamp", "Chair", "Whiteboard", "Planner", "Holder"}},
	{"Outdoor", []string{"Tent", "Backpack", "Lantern", "Hammock", "Bottle", "Cooler", "Grill", "Umbrella", "Chair", "Tarp", "Stove", "Compass"}},
	{"Audio", []string{"Headphones", "Speaker", "Microphone", "Amplifier", "Turntable", "Earbuds", "Soundbar", "Radio", "Mixer", "Cable", "Stand", "Receiver"}},
	{"Fitness", []string{"Mat", "Dumbbell", "Kettlebell", "Band", "Roller", "Bench", "Rope", "Bottle", "Tracker", "Gloves", "Ball", "Bag"}},
	{"Garden", []string{"Hose", "Shovel", "Rake", "Pot", "Sprinkler", "Trowel", "Pruner", "Bench", "Birdhouse", "Lantern", "Planter", "Wheelbarrow"}},
	{"Travel", []string{"Suitcase", "Duffel", "Pillow", "Organizer", "Adapter", "Wallet", "Tag", "Pouch", "Backpack", "Scale", "Lock", "Kit"}},
}

var adjectives = []string{
	"Compact", "Classic", "Modern", "Rustic", "Minimal", "Premium", "Portable", "Sturdy", "Vintage", "Sleek",
	"Cozy", "Bold", "Smart", "Nordic", "Urban", "Handmade", "Ergonomic", "Foldable", "Deluxe", "Essential",
	"Quiet", "Bright", "Soft", "Heavy-Duty", "Lightweight", "Elegant", "Everyday", "Pro", "Retro", "Studio",
}

var materials = []string{
	"Oak", "Bamboo", "Steel", "Ceramic", "Linen", "Walnut", "Aluminum", "Glass", "Cotton", "Leather",
	"Copper", "Wool", "Marble", "Cork", "Rattan", "Titanium", "Canvas", "Stone", "Brass", "Recycled",
}

var firstNames = []string{
	"Ana", "Bruno", "Carla", "Diego", "Elisa", "Felipe", "Gabriela", "Hugo", "Isabela", "João",
	"Karina", "Lucas", "Marina", "Nicolas", "Olivia", "Pedro", "Rafaela", "Samuel", "Tatiana", "Vitor",
}

var lastNames = []string{
	"Almeida", "Barbosa", "Cardoso", "Dias", "Esteves", "Ferreira", "Gomes", "Henriques", "Lima", "Martins",
	"Nunes", "Oliveira", "Pereira", "Ribeiro", "Santos", "Teixeira", "Vieira", "Xavier", "Moreira", "Rocha",
}

var sentences = []string{
	"Designed to last for years of daily use.",
	"Ships in recyclable packaging.",
	"Easy to clean and simple to store.",
	"A customer favorite since launch.",
	"Pairs well with the rest of the collection.",
	"Backed by a two-year warranty.",
	"Tested by our team before every batch.",
	"Made in small batches by independent makers.",
	"Available while stock lasts.",
	"Great as a gift.",
}

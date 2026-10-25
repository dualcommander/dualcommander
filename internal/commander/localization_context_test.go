package commander

import (
	"encoding/json"
	"testing"
)

// Regression examples for ambiguous English words: commands are actions, folders
// are directories, and archive passwords unlock packages rather than archive text.
func TestFileManagerTranslationContext(t *testing.T) {
	for _, tc := range []struct{ locale, key, want string }{
		{"cs", "Close", "Zavřít"},
		{"da", "View", "Vis"},
		{"fi", "Archive password", "Arkiston salasana"},
		{"hu", "Close", "Bezárás"},
		{"it", "Target", "Destinazione"},
		{"it", "Mark item and show size", "Contrassegna l’elemento e mostra la dimensione"},
		{"pl", "Folder", "Folder"},
		{"ro", "Folder", "Dosar"},
		{"nl", "Close", "Sluiten"},
		{"nl", "Mark item and show size", "Item markeren en grootte tonen"},
		{"pl", "Copy", "Kopiuj"},
		{"ro", "Close", "Închide"},
		{"sv", "Cancel", "Avbryt"},
		{"th", "Search in archives", "ค้นหาในไฟล์บีบอัด"},
		{"tr", "Folder", "Klasör"},
		{"tr", "Archive password", "Arşiv parolası"},
		{"uk", "Copy", "Копіювати"},
		{"vi", "File", "Tệp"},
		{"vi", "Skip", "Bỏ qua"},
		{"bn", " Overwrite merges folders; conflicting children are checked individually.", " ওভাররাইট করলে ফোল্ডারগুলো একত্রিত হয়; ভেতরের ফাইল ও সাবফোল্ডারের নামের সংঘাত আলাদাভাবে পরীক্ষা করা হয়।"},
		{"sk", " Overwrite merges folders; conflicting children are checked individually.", " Pri prepísaní sa priečinky zlúčia; konflikty vnorených súborov a priečinkov sa kontrolujú jednotlivo."},
		{"sw", "Archive password", "Nenosiri la faili la kumbukumbu"},
		{"ta", "Archive password", "காப்பகக் கடவுச்சொல்"},
		{"te", "Archive password", "ఆర్కైవ్ సంకేతపదం"},
	} {
		t.Run(tc.locale+"/"+tc.key, func(t *testing.T) {
			data, err := translations.ReadFile("translations/" + tc.locale + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var messages map[string]string
			if err := json.Unmarshal(data, &messages); err != nil {
				t.Fatal(err)
			}
			if got := messages[tc.key]; got != tc.want {
				t.Errorf("%q: got %q, want file-manager wording %q", tc.key, got, tc.want)
			}
		})
	}
}

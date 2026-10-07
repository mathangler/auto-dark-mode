package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// City is an entry in the built-in picker table. Name is the canonical
// (mostly English/romanized) key used for storage and matching; NameZh is the
// Chinese display name used when the UI language is Chinese.
type City struct {
	Name   string  `json:"name"`
	NameZh string  `json:"nameZh"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
}

// LocationResult reports a resolved coordinate and how it was obtained.
type LocationResult struct {
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Source string  `json:"source"` // "gps" | "ip" | "manual" | "city"
	Name   string  `json:"name"`
	NameZh string  `json:"nameZh,omitempty"`
	Error  string  `json:"error,omitempty"`
}

var cities = []City{
	// --- China (provincial capitals + major cities) ---
	{"Beijing", "北京", 39.9042, 116.4074},
	{"Shanghai", "上海", 31.2304, 121.4737},
	{"Tianjin", "天津", 39.3434, 117.3616},
	{"Chongqing", "重庆", 29.5630, 106.5516},
	{"Guangzhou", "广州", 23.1291, 113.2644},
	{"Shenzhen", "深圳", 22.5431, 114.0579},
	{"Hangzhou", "杭州", 30.2741, 120.1551},
	{"Nanjing", "南京", 32.0603, 118.7969},
	{"Suzhou", "苏州", 31.2989, 120.5853},
	{"Wuxi", "无锡", 31.4912, 120.3119},
	{"Ningbo", "宁波", 29.8683, 121.5440},
	{"Wuhan", "武汉", 30.5928, 114.3055},
	{"Chengdu", "成都", 30.5728, 104.0668},
	{"Xi'an", "西安", 34.3416, 108.9398},
	{"Harbin", "哈尔滨", 45.8038, 126.5350},
	{"Shenyang", "沈阳", 41.8057, 123.4315},
	{"Changchun", "长春", 43.8171, 125.3235},
	{"Dalian", "大连", 38.9140, 121.6147},
	{"Jinan", "济南", 36.6512, 117.1201},
	{"Qingdao", "青岛", 36.0671, 120.3826},
	{"Zhengzhou", "郑州", 34.7466, 113.6254},
	{"Changsha", "长沙", 28.2282, 112.9388},
	{"Fuzhou", "福州", 26.0745, 119.2965},
	{"Xiamen", "厦门", 24.4798, 118.0894},
	{"Hefei", "合肥", 31.8206, 117.2272},
	{"Nanchang", "南昌", 28.6820, 115.8579},
	{"Kunming", "昆明", 25.0389, 102.7183},
	{"Guiyang", "贵阳", 26.6470, 106.6302},
	{"Nanning", "南宁", 22.8170, 108.3665},
	{"Haikou", "海口", 20.0440, 110.1999},
	{"Sanya", "三亚", 18.2528, 109.5119},
	{"Lhasa", "拉萨", 29.6520, 91.1721},
	{"Lanzhou", "兰州", 36.0611, 103.8343},
	{"Xining", "西宁", 36.6171, 101.7782},
	{"Yinchuan", "银川", 38.4872, 106.2309},
	{"Urumqi", "乌鲁木齐", 43.8256, 87.6168},
	{"Hohhot", "呼和浩特", 40.8424, 111.7496},
	{"Shijiazhuang", "石家庄", 38.0428, 114.5149},
	{"Taiyuan", "太原", 37.8706, 112.5489},
	{"Foshan", "佛山", 23.0215, 113.1214},
	{"Dongguan", "东莞", 23.0207, 113.7518},
	{"Zhuhai", "珠海", 22.2707, 113.5767},
	{"Zhongshan", "中山", 22.5176, 113.3928},
	{"Hong Kong", "香港", 22.3193, 114.1694},
	{"Macau", "澳门", 22.1987, 113.5439},
	{"Taipei", "台北", 25.0330, 121.5654},

	// --- Japan / Korea ---
	{"Tokyo", "东京", 35.6762, 139.6503},
	{"Osaka", "大阪", 34.6937, 135.5023},
	{"Kyoto", "京都", 35.0116, 135.7681},
	{"Nagoya", "名古屋", 35.1815, 136.9066},
	{"Sapporo", "札幌", 43.0618, 141.3545},
	{"Hiroshima", "广岛", 34.3853, 132.4553},
	{"Seoul", "首尔", 37.5665, 126.9780},
	{"Busan", "釜山", 35.1796, 129.0756},
	{"Incheon", "仁川", 37.4563, 126.7052},

	// --- South / Southeast Asia ---
	{"Mumbai", "孟买", 19.0760, 72.8777},
	{"Delhi", "德里", 28.7041, 77.1025},
	{"Bangalore", "班加罗尔", 12.9716, 77.5946},
	{"Chennai", "金奈", 13.0827, 80.2707},
	{"Kolkata", "加尔各答", 22.5726, 88.3639},
	{"Hyderabad", "海得拉巴", 17.3850, 78.4867},
	{"Pune", "浦那", 18.5204, 73.8567},
	{"Karachi", "卡拉奇", 24.8607, 67.0011},
	{"Lahore", "拉合尔", 31.5204, 74.3587},
	{"Islamabad", "伊斯兰堡", 33.6844, 73.0479},
	{"Dhaka", "达卡", 23.8103, 90.4125},
	{"Kathmandu", "加德满都", 27.7172, 85.3240},
	{"Colombo", "科伦坡", 6.9271, 79.8612},
	{"Singapore", "新加坡", 1.3521, 103.8198},
	{"Kuala Lumpur", "吉隆坡", 3.1390, 101.6869},
	{"Bangkok", "曼谷", 13.7563, 100.5018},
	{"Ho Chi Minh City", "胡志明市", 10.8231, 106.6297},
	{"Hanoi", "河内", 21.0278, 105.8342},
	{"Jakarta", "雅加达", -6.2088, 106.8456},
	{"Manila", "马尼拉", 14.5995, 120.9842},
	{"Yangon", "仰光", 16.8409, 96.1735},
	{"Phnom Penh", "金边", 11.5564, 104.9282},
	{"Kabul", "喀布尔", 34.5553, 69.2075},

	// --- Middle East / Central Asia ---
	{"Tehran", "德黑兰", 35.6892, 51.3890},
	{"Baghdad", "巴格达", 33.3152, 44.3661},
	{"Riyadh", "利雅得", 24.7136, 46.6753},
	{"Jeddah", "吉达", 21.4858, 39.1925},
	{"Dubai", "迪拜", 25.2048, 55.2708},
	{"Abu Dhabi", "阿布扎比", 24.4539, 54.3773},
	{"Doha", "多哈", 25.2854, 51.5310},
	{"Kuwait City", "科威特城", 29.3759, 47.9774},
	{"Tel Aviv", "特拉维夫", 32.0853, 34.7818},
	{"Jerusalem", "耶路撒冷", 31.7683, 35.2137},
	{"Amman", "安曼", 31.9454, 35.9284},
	{"Sana'a", "萨那", 15.3694, 44.1910},
	{"Tashkent", "塔什干", 41.2995, 69.2401},
	{"Almaty", "阿拉木图", 43.2220, 76.8512},
	{"Baku", "巴库", 40.4093, 49.8671},
	{"Tbilisi", "第比利斯", 41.7151, 44.8271},

	// --- Europe ---
	{"London", "伦敦", 51.5074, -0.1278},
	{"Paris", "巴黎", 48.8566, 2.3522},
	{"Berlin", "柏林", 52.5200, 13.4050},
	{"Madrid", "马德里", 40.4168, -3.7038},
	{"Barcelona", "巴塞罗那", 41.3874, 2.1686},
	{"Lisbon", "里斯本", 38.7223, -9.1393},
	{"Rome", "罗马", 41.9028, 12.4964},
	{"Milan", "米兰", 45.4642, 9.1900},
	{"Naples", "那不勒斯", 40.8518, 14.2681},
	{"Amsterdam", "阿姆斯特丹", 52.3676, 4.9041},
	{"Rotterdam", "鹿特丹", 51.9244, 4.4777},
	{"Brussels", "布鲁塞尔", 50.8503, 4.3517},
	{"Vienna", "维也纳", 48.2082, 16.3738},
	{"Zurich", "苏黎世", 47.3769, 8.5417},
	{"Geneva", "日内瓦", 46.2044, 6.1432},
	{"Munich", "慕尼黑", 48.1351, 11.5820},
	{"Frankfurt", "法兰克福", 50.1109, 8.6821},
	{"Stockholm", "斯德哥尔摩", 59.3293, 18.0686},
	{"Oslo", "奥斯陆", 59.9139, 10.7522},
	{"Copenhagen", "哥本哈根", 55.6761, 12.5683},
	{"Helsinki", "赫尔辛基", 60.1699, 24.9384},
	{"Dublin", "都柏林", 53.3498, -6.2603},
	{"Edinburgh", "爱丁堡", 55.9533, -3.1883},
	{"Warsaw", "华沙", 52.2297, 21.0122},
	{"Prague", "布拉格", 50.0755, 14.4378},
	{"Budapest", "布达佩斯", 47.4979, 19.0402},
	{"Bucharest", "布加勒斯特", 44.4268, 26.1025},
	{"Sofia", "索非亚", 42.6977, 23.3219},
	{"Belgrade", "贝尔格莱德", 44.7866, 20.4489},
	{"Zagreb", "萨格勒布", 45.8150, 15.9819},
	{"Kyiv", "基辅", 50.4501, 30.5234},
	{"Moscow", "莫斯科", 55.7558, 37.6173},
	{"St Petersburg", "圣彼得堡", 59.9311, 30.3609},
	{"Yekaterinburg", "叶卡捷琳堡", 56.8389, 60.6057},
	{"Novosibirsk", "新西伯利亚", 55.0084, 82.9357},
	{"Vladivostok", "符拉迪沃斯托克", 43.1332, 131.9113},

	// --- Africa ---
	{"Cairo", "开罗", 30.0444, 31.2357},
	{"Alexandria", "亚历山大", 31.2001, 29.9187},
	{"Casablanca", "卡萨布兰卡", 33.5731, -7.5898},
	{"Algiers", "阿尔及尔", 36.7538, 3.0588},
	{"Tunis", "突尼斯", 36.8065, 10.1815},
	{"Tripoli", "的黎波里", 32.8872, 13.1913},
	{"Nairobi", "内罗毕", -1.2921, 36.8219},
	{"Lagos", "拉各斯", 6.5244, 3.3792},
	{"Accra", "阿克拉", 5.6037, -0.1870},
	{"Abuja", "阿布贾", 9.0765, 7.3986},
	{"Kinshasa", "金沙萨", -4.4419, 15.2663},
	{"Addis Ababa", "亚的斯亚贝巴", 9.0054, 38.7636},
	{"Khartoum", "喀土穆", 15.5007, 32.5599},
	{"Johannesburg", "约翰内斯堡", -26.2041, 28.0473},
	{"Cape Town", "开普敦", -33.9249, 18.4241},
	{"Durban", "德班", -29.8587, 31.0218},
	{"Antananarivo", "塔那那利佛", -18.8792, 47.5079},

	// --- North America ---
	{"New York", "纽约", 40.7128, -74.0060},
	{"Los Angeles", "洛杉矶", 34.0522, -118.2437},
	{"Chicago", "芝加哥", 41.8781, -87.6298},
	{"Houston", "休斯顿", 29.7604, -95.3698},
	{"Phoenix", "菲尼克斯", 33.4484, -112.0740},
	{"Philadelphia", "费城", 39.9526, -75.1652},
	{"San Antonio", "圣安东尼奥", 29.4241, -98.4936},
	{"San Diego", "圣迭戈", 32.7157, -117.1611},
	{"Dallas", "达拉斯", 32.7767, -96.7970},
	{"San Francisco", "旧金山", 37.7749, -122.4194},
	{"Seattle", "西雅图", 47.6062, -122.3321},
	{"Miami", "迈阿密", 25.7617, -80.1918},
	{"Boston", "波士顿", 42.3601, -71.0589},
	{"Washington, D.C.", "华盛顿", 38.9072, -77.0369},
	{"Atlanta", "亚特兰大", 33.7490, -84.3880},
	{"Denver", "丹佛", 39.7392, -104.9903},
	{"Las Vegas", "拉斯维加斯", 36.1699, -115.1398},
	{"Portland", "波特兰", 45.5152, -122.6784},
	{"Toronto", "多伦多", 43.6532, -79.3832},
	{"Montreal", "蒙特利尔", 45.5017, -73.5673},
	{"Vancouver", "温哥华", 49.2827, -123.1207},
	{"Ottawa", "渥太华", 45.4215, -75.6972},
	{"Calgary", "卡尔加里", 51.0447, -114.0719},
	{"Edmonton", "埃德蒙顿", 53.5461, -113.4938},
	{"Quebec City", "魁北克城", 46.8139, -71.2080},
	{"Mexico City", "墨西哥城", 19.4326, -99.1332},
	{"Guadalajara", "瓜达拉哈拉", 20.6597, -103.3496},
	{"Monterrey", "蒙特雷", 25.6866, -100.3161},
	{"Cancun", "坎昆", 21.1619, -86.8515},
	{"Havana", "哈瓦那", 23.1136, -82.3666},
	{"San Juan", "圣胡安", 18.4655, -66.1057},
	{"Kingston", "金斯敦", 18.0179, -76.8099},
	{"Panama City", "巴拿马城", 8.9824, -79.5199},
	{"San Jose", "圣何塞", 9.9281, -84.0907},
	{"Honolulu", "檀香山", 21.3069, -157.8583},
	{"Anchorage", "安克雷奇", 61.2181, -149.9003},

	// --- South America ---
	{"Bogota", "波哥大", 4.7110, -74.0721},
	{"Medellin", "麦德林", 6.2442, -75.5812},
	{"Caracas", "加拉加斯", 10.4806, -66.9036},
	{"Quito", "基多", -0.1807, -78.4678},
	{"Guayaquil", "瓜亚基尔", -2.1890, -79.8891},
	{"Lima", "利马", -12.0464, -77.0428},
	{"La Paz", "拉巴斯", -16.5000, -68.1500},
	{"Santiago", "圣地亚哥", -33.4489, -70.6693},
	{"Buenos Aires", "布宜诺斯艾利斯", -34.6037, -58.3816},
	{"Cordoba", "科尔多瓦", -31.4201, -64.1888},
	{"Montevideo", "蒙得维的亚", -34.9011, -56.1645},
	{"Asuncion", "亚松森", -25.2637, -57.5759},
	{"Sao Paulo", "圣保罗", -23.5505, -46.6333},
	{"Rio de Janeiro", "里约热内卢", -22.9068, -43.1729},
	{"Brasilia", "巴西利亚", -15.8267, -47.9218},
	{"Salvador", "萨尔瓦多", -12.9777, -38.5016},
	{"Recife", "累西腓", -8.0476, -34.8770},
	{"Fortaleza", "福塔莱萨", -3.7319, -38.5267},
	{"Belo Horizonte", "贝洛哈里桑塔", -19.9167, -43.9345},

	// --- Oceania ---
	{"Sydney", "悉尼", -33.8688, 151.2093},
	{"Melbourne", "墨尔本", -37.8136, 144.9631},
	{"Brisbane", "布里斯班", -27.4698, 153.0251},
	{"Perth", "珀斯", -31.9505, 115.8605},
	{"Adelaide", "阿德莱德", -34.9285, 138.6007},
	{"Canberra", "堪培拉", -35.2809, 149.1300},
	{"Hobart", "霍巴特", -42.8821, 147.3272},
	{"Auckland", "奥克兰", -36.8485, 174.7633},
	{"Wellington", "惠灵顿", -41.2866, 174.7756},
	{"Christchurch", "基督城", -43.5321, 172.6362},
}

func cityByName(name string) (City, bool) {
	for _, c := range cities {
		if c.Name == name {
			return c, true
		}
	}
	return City{}, false
}

type ipGeo struct {
	Status string  `json:"status"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	City   string  `json:"city"`
	Query  string  `json:"query"`
}

type ipInfo struct {
	City string `json:"city"`
	Loc  string `json:"loc"` // "lat,lon"
}

// queryIPAPI estimates coordinates from the public IP via ip-api.com.
func queryIPAPI(ctx context.Context) (LocationResult, error) {
	cli := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://ip-api.com/json/?fields=status,lat,lon,city", nil)
	if err != nil {
		return LocationResult{}, err
	}
	req.Header.Set("User-Agent", "auto-dark-mode/1.0")
	resp, err := cli.Do(req)
	if err != nil {
		return LocationResult{}, err
	}
	defer resp.Body.Close()
	var g ipGeo
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return LocationResult{}, err
	}
	if g.Status != "success" {
		return LocationResult{}, fmt.Errorf("geolocation failed: %s", g.Status)
	}
	return LocationResult{Lat: g.Lat, Lon: g.Lon, Source: "ip", Name: g.City}, nil
}

// queryIPInfo estimates coordinates from the public IP via ipinfo.io.
func queryIPInfo(ctx context.Context) (LocationResult, error) {
	cli := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ipinfo.io/json", nil)
	if err != nil {
		return LocationResult{}, err
	}
	req.Header.Set("User-Agent", "auto-dark-mode/1.0")
	resp, err := cli.Do(req)
	if err != nil {
		return LocationResult{}, err
	}
	defer resp.Body.Close()
	var g ipInfo
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return LocationResult{}, err
	}
	parts := strings.Split(g.Loc, ",")
	if len(parts) != 2 {
		return LocationResult{}, fmt.Errorf("bad ipinfo loc: %q", g.Loc)
	}
	var lat, lon float64
	if _, err := fmt.Sscanf(parts[0], "%f", &lat); err != nil {
		return LocationResult{}, err
	}
	if _, err := fmt.Sscanf(parts[1], "%f", &lon); err != nil {
		return LocationResult{}, err
	}
	return LocationResult{Lat: lat, Lon: lon, Source: "ip", Name: g.City}, nil
}

// locateByIP estimates coordinates from the public IP address, trying
// multiple free services for resilience.
func locateByIP(ctx context.Context) (LocationResult, error) {
	if r, err := queryIPAPI(ctx); err == nil {
		return r, nil
	} else if r, ipErr := queryIPInfo(ctx); ipErr == nil {
		return r, nil
	} else {
		return LocationResult{}, err
	}
}

// errGPSDenied signals that the user did not grant Windows location access.
var errGPSDenied = errors.New("gps location access denied")

// runHiddenPS runs a PowerShell snippet with a hidden window and returns its
// combined output.
func runHiddenPS(ctx context.Context, script string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	// CREATE_NO_WINDOW prevents a console window from flashing while probing GPS.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// gpsRequestAccess asks Windows for location permission and reports whether
// access was granted.
func gpsRequestAccess(ctx context.Context) (bool, error) {
	script := `
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { $_.Name -eq 'AsTask' -and $_.IsGenericMethod -and $_.GetParameters().Count -eq 1 })[0]
function Await($WinRtTask, $ResultType) {
  $asTask = $asTaskGeneric.MakeGenericMethod($ResultType)
  $netTask = $asTask.Invoke($null, @($WinRtTask))
  $netTask.Wait(-1) | Out-Null
  $netTask.Result
}
$null = [Windows.Devices.Geolocation.Geolocator, Windows.Devices.Geolocation, ContentType = WindowsRuntime]
$access = Await ([Windows.Devices.Geolocation.Geolocator]::RequestAccessAsync()) ([Windows.Devices.Geolocation.GeolocationAccessStatus])
Write-Output $access
`
	out, err := runHiddenPS(ctx, script)
	if err != nil {
		return false, err
	}
	return out == "Allowed", nil
}

// gpsGetPosition gets a position, assuming location access was granted.
func gpsGetPosition(ctx context.Context) (LocationResult, error) {
	script := `
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { $_.Name -eq 'AsTask' -and $_.IsGenericMethod -and $_.GetParameters().Count -eq 1 })[0]
function Await($WinRtTask, $ResultType) {
  $asTask = $asTaskGeneric.MakeGenericMethod($ResultType)
  $netTask = $asTask.Invoke($null, @($WinRtTask))
  $netTask.Wait(-1) | Out-Null
  $netTask.Result
}
$null = [Windows.Devices.Geolocation.Geolocator, Windows.Devices.Geolocation, ContentType = WindowsRuntime]
$geo = New-Object Windows.Devices.Geolocation.Geolocator
$pos = Await ($geo.GetGeopositionAsync()) ([Windows.Devices.Geolocation.Geoposition])
$lat = $pos.Coordinate.Point.Position.Latitude
$lon = $pos.Coordinate.Point.Position.Longitude
Write-Output ("{0:F5},{1:F5}" -f $lat, $lon)
`
	out, err := runHiddenPS(ctx, script)
	if err != nil {
		return LocationResult{}, err
	}
	parts := strings.Split(out, ",")
	if len(parts) != 2 {
		return LocationResult{}, fmt.Errorf("unexpected GPS output: %q", out)
	}
	var lat, lon float64
	if _, err := fmt.Sscanf(parts[0], "%f", &lat); err != nil {
		return LocationResult{}, err
	}
	if _, err := fmt.Sscanf(parts[1], "%f", &lon); err != nil {
		return LocationResult{}, err
	}
	return LocationResult{Lat: lat, Lon: lon, Source: "gps", Name: "GPS"}, nil
}

// locateByGPS resolves a position via the Windows location service, requesting
// permission first. Used by the background scheduler; returns errGPSDenied if
// the user denied access so callers can fall back gracefully.
func locateByGPS(ctx context.Context) (LocationResult, error) {
	allowed, err := gpsRequestAccess(ctx)
	if err != nil {
		return LocationResult{}, err
	}
	if !allowed {
		return LocationResult{}, errGPSDenied
	}
	return gpsGetPosition(ctx)
}

// resolveLocation determines coordinates following the configured source:
// auto tries GPS (requesting permission), then IP geolocation, then reports a
// clear "set it manually" message. manual and city read directly from config.
// Caller is responsible for caching the "auto" result.
func (a *App) resolveLocation() LocationResult {
	cfg := a.cfg
	lang := effectiveLang(cfg)
	ctx := context.Background()
	switch cfg.LocationSource {
	case SourceManual:
		return LocationResult{Lat: cfg.Lat, Lon: cfg.Lon, Source: "manual", Name: "Manual"}
	case SourceCity:
		if c, ok := cityByName(cfg.City); ok {
			return LocationResult{Lat: c.Lat, Lon: c.Lon, Source: "city", Name: c.Name, NameZh: c.NameZh}
		}
		return LocationResult{Error: T(lang, "err.unknownCity")}
	default: // auto: GPS then IP
		if r, gpsErr := locateByGPS(ctx); gpsErr == nil {
			return r
		} else if r, ipErr := locateByIP(ctx); ipErr == nil {
			return r
		} else {
			return LocationResult{
				Error: fmt.Sprintf("%s (%s)", T(lang, "err.noLocation"), ipErr.Error()),
			}
		}
	}
}

// --- auto-location disk cache ---------------------------------------------
//
// The GPS+IP probe chain can take tens of seconds. currentLocation() runs
// with the app mutex held (and once on the startup tick), and Wails serves
// both the window assets and the JS bindings from the same main loop — a
// synchronous resolve there froze the whole UI (the "black window for ~30s
// after install" bug). The rule this cache enforces: the auto path NEVER
// resolves synchronously; it answers from disk instantly and the probe runs
// in the background.

type locationCache struct {
	LocationResult
	At time.Time `json:"at"`
}

func locationCachePath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "AutoDarkMode", "location_cache.json")
}

// loadLocationCache returns the last successful auto-resolve, if any.
func loadLocationCache() (locationCache, bool) {
	var c locationCache
	b, err := os.ReadFile(locationCachePath())
	if err != nil {
		return c, false
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return locationCache{}, false
	}
	if c.Source == "" || c.Error != "" {
		return locationCache{}, false
	}
	return c, true
}

// saveLocationCache persists a successful auto-resolve. Best-effort: a cache
// write failure must never surface as an app error.
func saveLocationCache(r LocationResult) {
	if r.Source == "" || r.Error != "" {
		return
	}
	b, err := json.Marshal(locationCache{LocationResult: r, At: time.Now()})
	if err != nil {
		return
	}
	p := locationCachePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(p, b, 0o644)
}

// coordJumpDegrees returns the straight-line distance between two fixes in
// (rough) degrees — plenty for the ~2° sanity threshold. Longitude is not
// scaled by cos(lat); at the latitudes in play that only makes the guard
// slightly more conservative, which is the safe direction.
func coordJumpDegrees(a, b LocationResult) float64 {
	return math.Hypot(a.Lat-b.Lat, a.Lon-b.Lon)
}

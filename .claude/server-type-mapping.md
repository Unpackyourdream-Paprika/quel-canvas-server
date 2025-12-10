# Go Server 모듈별 Type 매핑 현황

## 📁 소스 파일 위치
- `modules/fashion/worker.go`
- `modules/beauty/worker.go`
- `modules/eats/worker.go`
- `modules/cinema/worker.go`
- `modules/cartoon/worker.go`

---

## Fashion 모듈

### switch case (직접 매핑)
| type | 분류 카테고리 |
|------|-------------|
| `model` | Model |
| `background`, `bg` | Background |

### clothingTypes (의류)
```go
clothingTypes := map[string]bool{"top": true, "pants": true, "outer": true}
```
- `top` → Clothing
- `pants` → Clothing
- `outer` → Clothing

### accessoryTypes (악세서리)
```go
accessoryTypes := map[string]bool{"shoes": true, "bag": true, "accessory": true, "acce": true}
```
- `shoes` → Accessories
- `bag` → Accessories
- `accessory` → Accessories
- `acce` → Accessories

### 기타 처리
- `none`, `product` → Accessories (기본 처리)
- 알 수 없는 type → 스킵

---

## Beauty 모듈

### switch case (직접 매핑)
| type | 분류 카테고리 |
|------|-------------|
| `model` | Model |
| `background`, `bg` | Background |

### productTypes (제품)
```go
productTypes := map[string]bool{
    "product":  true,
    "lipstick": true,
    "cream":    true,
    "bottle":   true,
    "compact":  true,
    "cosmetic": true,
    "skincare": true,
    "makeup":   true,
}
```
- `product` → Products
- `lipstick` → Products
- `cream` → Products
- `bottle` → Products
- `compact` → Products
- `cosmetic` → Products
- `skincare` → Products
- `makeup` → Products

### accessoryTypes (악세서리)
```go
accessoryTypes := map[string]bool{"brush": true, "tool": true, "acce": true}
```
- `brush` → Accessories
- `tool` → Accessories
- `acce` → Accessories

### 기타 처리
- 알 수 없는 type → Products (기본 처리)

---

## Eats 모듈

### switch case (직접 매핑)
| type | 분류 카테고리 |
|------|-------------|
| `model`, `food`, `dish`, `main`, `product` | Model (메인 음식) |
| `background`, `bg` | Background |

### clothingTypes (재료 - Eats에서는 부재료로 사용)
```go
clothingTypes := map[string]bool{"top": true, "pants": true, "outer": true, "ingredient": true, "side": true}
```
- `ingredient` → Clothing (부재료)
- `side` → Clothing (사이드)

### accessoryTypes (토핑/장식)
```go
accessoryTypes := map[string]bool{"shoes": true, "bag": true, "accessory": true, "acce": true, "topping": true, "garnish": true, "prop": true}
```
- `topping` → Accessories
- `garnish` → Accessories
- `prop` → Accessories

### Pipeline Stage 전용 (ingredientTypes, toppingTypes)
```go
ingredientTypes := map[string]bool{"ingredient": true, "side": true}
toppingTypes := map[string]bool{"topping": true, "garnish": true, "prop": true}
```

---

## Cinema 모듈

### switch case (직접 매핑)
| type | 분류 카테고리 |
|------|-------------|
| `model`, `character`, `actor`, `face` | Models (최대 4명) |
| `background`, `bg` | Background |

### clothingTypes (의류)
```go
clothingTypes := map[string]bool{"top": true, "pants": true, "outer": true}
```
- `top` → Clothing
- `pants` → Clothing
- `outer` → Clothing

### accessoryTypes (악세서리/소품)
```go
accessoryTypes := map[string]bool{"shoes": true, "bag": true, "accessory": true, "acce": true, "prop": true}
```
- `shoes` → Accessories
- `bag` → Accessories
- `accessory` → Accessories
- `acce` → Accessories
- `prop` → Accessories

---

## Cartoon 모듈

### switch case (직접 매핑)
| type | 분류 카테고리 |
|------|-------------|
| `model`, `character`, `face` | Models (최대 4명) |
| `background`, `bg` | Background |

### clothingTypes (의류)
```go
clothingTypes := map[string]bool{"top": true, "pants": true, "outer": true}
```
- `top` → Clothing
- `pants` → Clothing
- `outer` → Clothing

### accessoryTypes (악세서리/소품)
```go
accessoryTypes := map[string]bool{"shoes": true, "bag": true, "accessory": true, "acce": true, "prop": true}
```
- `shoes` → Accessories
- `bag` → Accessories
- `accessory` → Accessories
- `acce` → Accessories
- `prop` → Accessories

---

## 전체 Type 지원 현황 요약

| type | Fashion | Beauty | Eats | Cinema | Cartoon |
|------|---------|--------|------|--------|---------|
| `none` | Accessories | - | - | - | - |
| `model` | Model | Model | Model | Models | Models |
| `top` | Clothing | - | - | Clothing | Clothing |
| `pants` | Clothing | - | - | Clothing | Clothing |
| `outer` | Clothing | - | - | Clothing | Clothing |
| `shoes` | Accessories | - | - | Accessories | Accessories |
| `bag` | Accessories | - | - | Accessories | Accessories |
| `acce` | Accessories | Accessories | Accessories | Accessories | Accessories |
| `accessory` | Accessories | - | Accessories | Accessories | Accessories |
| `background` | Background | Background | Background | Background | Background |
| `bg` | Background | Background | Background | Background | Background |
| `product` | Accessories | Products | Model | - | - |
| `food` | - | - | Model | - | - |
| `dish` | - | - | Model | - | - |
| `main` | - | - | Model | - | - |
| `ingredient` | - | - | Clothing | - | - |
| `side` | - | - | Clothing | - | - |
| `topping` | - | - | Accessories | - | - |
| `garnish` | - | - | Accessories | - | - |
| `prop` | - | - | Accessories | Accessories | Accessories |
| `actor` | - | - | - | Models | - |
| `face` | - | - | - | Models | Models |
| `character` | - | - | - | Models | Models |
| `lipstick` | - | Products | - | - | - |
| `cream` | - | Products | - | - | - |
| `bottle` | - | Products | - | - | - |
| `compact` | - | Products | - | - | - |
| `cosmetic` | - | Products | - | - | - |
| `skincare` | - | Products | - | - | - |
| `makeup` | - | Products | - | - | - |
| `brush` | - | Accessories | - | - | - |
| `tool` | - | Accessories | - | - | - |

---

**작성일**: 2025-12-10

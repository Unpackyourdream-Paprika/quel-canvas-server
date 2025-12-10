# 이미지 카테고리 드롭다운 옵션

## 📁 소스 파일
`src/components/nodes/GroupNode.tsx` - getCategoryOptions 함수

---

## 페이지별 드롭다운 옵션

### Fashion (기본값)
```typescript
case "fashion":
default:
  return [
    { value: "none", label: "Unset" },
    { value: "model", label: "Model" },
    { value: "top", label: "Top" },
    { value: "pants", label: "Pants" },
    { value: "outer", label: "Outer" },
    { value: "shoes", label: "Shoes" },
    { value: "bag", label: "Bag" },
    { value: "acce", label: "Accessory" },
    { value: "background", label: "Background" },
  ];
```

### Beauty
```typescript
case "beauty":
  return [
    { value: "none", label: "Unset" },
    { value: "product", label: "Product" },
    { value: "model", label: "Model" },
    { value: "background", label: "Background" },
  ];
```

### Eats/Food
```typescript
case "food":
case "eats":
  return [
    { value: "none", label: "Unset" },
    { value: "food", label: "Food/Dish" },
    { value: "ingredient", label: "Ingredient" },
    { value: "prop", label: "Prop" },
    { value: "background", label: "Background" },
  ];
```

### Cinema
```typescript
case "cinema":
case "drama":
case "film":
case "movie":
case "advertisement":
  return [
    { value: "none", label: "Unset" },
    { value: "actor", label: "Actor" },
    { value: "top", label: "Top" },
    { value: "pants", label: "Pants" },
    { value: "outer", label: "Outer" },
    { value: "face", label: "Face Reference" },
    { value: "prop", label: "Prop" },
    { value: "background", label: "Background" },
  ];
```

### Cartoon
```typescript
case "cartoon":
case "animation":
  return [
    { value: "none", label: "Unset" },
    { value: "character", label: "Character" },
    { value: "face", label: "Face Reference" },
    { value: "prop", label: "Prop" },
    { value: "background", label: "Background" },
  ];
```

### Interior
```typescript
case "interior":
  return [
    { value: "none", label: "Unset" },
    { value: "product", label: "Product" },
    { value: "prop", label: "Prop" },
    { value: "background", label: "Background" },
  ];
```

---

## 전체 value 목록 (중복 제거)

| value | 사용하는 페이지 |
|-------|---------------|
| `none` | 전체 |
| `model` | Fashion, Beauty |
| `top` | Fashion, Cinema |
| `pants` | Fashion, Cinema |
| `outer` | Fashion, Cinema |
| `shoes` | Fashion |
| `bag` | Fashion |
| `acce` | Fashion |
| `background` | 전체 |
| `product` | Beauty, Interior |
| `food` | Eats |
| `ingredient` | Eats |
| `prop` | Eats, Cinema, Cartoon, Interior |
| `actor` | Cinema |
| `face` | Cinema, Cartoon |
| `character` | Cartoon |

---

## Job 전송 시 individualImageAttachIds 구조

```json
{
  "individualImageAttachIds": [
    { "attachId": 123, "type": "food" },
    { "attachId": 456, "type": "background" }
  ]
}
```

**type 필드**: 드롭다운에서 선택한 `value` 값이 그대로 들어감. 선택 안 하면 `none`.

---

## Go Server 처리 필요 type 목록

Go Server에서 처리해야 할 type 값:

- `none` - 미선택
- `model` - 모델
- `top` - 상의
- `pants` - 하의
- `outer` - 아우터
- `shoes` - 신발
- `bag` - 가방
- `acce` - 악세서리
- `background` - 배경
- `product` - 제품
- `food` - 음식/요리
- `ingredient` - 재료
- `prop` - 소품
- `actor` - 배우
- `face` - 얼굴 레퍼런스
- `character` - 캐릭터

---

**작성일**: 2025-12-10

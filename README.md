
# TerminoGrid

Lightweight Docker web UI: manage containers, 1-click start/stop, unlimited terminals (tabs/grid).

---

## Features
- 🔹 Manage Docker containers (start/stop/restart/remove)
- 🔹 Unlimited terminal sessions per container
- 🔹 Grid & tab layout for shells
- 🔹 Fit Grid & Empty Grid mode
- 🔹 Live connection indicator

---

## Screenshots

### 🖥️ Overview
![Overview](https://raw.githubusercontent.com/Abbas-Hoseiny/terminogrid/main/docs/screenshots/overview.png)

### 🗂️ Grid View
![Grid View](https://raw.githubusercontent.com/Abbas-Hoseiny/terminogrid/main/docs/screenshots/overview-grid.png)

### 🍎 macOS Example
![macOS](https://raw.githubusercontent.com/Abbas-Hoseiny/terminogrid/main/docs/screenshots/macOS.png)

### 🪟 Windows Example
![Windows](https://raw.githubusercontent.com/Abbas-Hoseiny/terminogrid/main/docs/screenshots/windows.png)

### ➕ Demo Views
![Demo](https://raw.githubusercontent.com/Abbas-Hoseiny/terminogrid/main/docs/screenshots/demo.png)
![Demo1](https://raw.githubusercontent.com/Abbas-Hoseiny/terminogrid/main/docs/screenshots/demo1.png)
![Demo2](https://raw.githubusercontent.com/Abbas-Hoseiny/terminogrid/main/docs/screenshots/demo2.png)
![Demo3](https://raw.githubusercontent.com/Abbas-Hoseiny/terminogrid/main/docs/screenshots/demo3.png)

### ⬜ Empty Grid Mode
![Empty Grid](https://raw.githubusercontent.com/Abbas-Hoseiny/terminogrid/main/docs/screenshots/empty-grid.png)

---

## Getting Started

```bash
git clone https://github.com/Abbas-Hoseiny/terminogrid.git
cd terminogrid
docker-compose up -d
````

Then open [http://localhost:8181/ui/](http://localhost:8181/ui/) 🎉

---

## Docker Hub

Pull the image:

```bash
docker pull hoseiny/terminogrid:latest
```

Run it:

```bash
docker run -d -p 8181:8181 -v /var/run/docker.sock:/var/run/docker.sock hoseiny/terminogrid:latest
```

---

## License

MIT License © Abbas Hoseiny

```



<p align="center">
  <img src="readme/logo.png" height="80" alt="WSP logo">
</p>
<p align="center">
    <em>Create a backend for your websites using a lightweight scripting language.</em>
</p>
<p align="center">
  <img src="https://img.shields.io/badge/status-archive-red" alt="Project Status">
  <img src="https://img.shields.io/github/license/BFSDK/Website-Processor" alt="License">
  <img src="https://img.shields.io/github/v/release/BFSDK/Website-Processor?include_prereleases" alt="GitHub release (latest by date including pre-releases)">
  <img src="https://img.shields.io/github/last-commit/BFSDK/Website-Processor" alt="GitHub last commit">
</p>

---

**Website processor** allows you to create simple backend scripts for your websites. It is NOT an alternative to PHP. This is my personal project, and it is not intended for commercial use.

## Technologies
* **GO** - Code, interpreter.
* **HTML5, CSS, JS and more** - WSP is embedded in HTML code.

## Code examples
_Hello, world!_
```
<?wsp
$println Hello, world!
!>
```
_Arguments parser_
```
<?wsp
;; Parse the name and age arguments: /?name=...&age=...
$println Hello, ?args:name! You are ?args:age years old?
!>
```
_1 .. 10_
```
<?wsp
;; Let's write numbers from 1 to 10 on the website itself, not in the console:
$mem add integer i 1
WHILE ?mem:i <= 10
  $p ?mem:i
  $mem ++ i
ENDWHILE
!>
```

## Deploy a website
WSP deploys a site from a folder by initially loading index.wsp. To deploy a site on localhost, you need to enter this command (after adding the WSP folder to Path):
```
wsp <path/to/folder> port
```
Specify the path to the website folder without quotation marks.

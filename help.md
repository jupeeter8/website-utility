# The Blog
## How Page Retrieval Works
- Get the latest post: `curl whereisanirudh.info`
- Blog pages are 0 indexed. The oldest page is at path `curl whereisanirudh.info/0`
- To get a list of all pages: `curl whereisanirudh.info/blog`
- To use a light theme use the `theme` query parameter `curl whereisanirudh.info/12?theme=light`

---
🏷 Page Metadata:  
Each page contains a Week number that corresponds to the page’s index.
Example:
> ### Heading
> - Week 2
> - Wed Jan 1 2025
> - blog

Here, Week 2 means the page’s index is 2.
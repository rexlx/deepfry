const dataList = document.getElementById('data-list');
const scrollbar = document.querySelector('.scrollbar');
const thumb = document.querySelector('.thumb');
const container = document.querySelector('.container');
const loader = document.getElementById('loader');
// let currentColor = '#3232a8';

const itemsPerPage = 50; // Reduce items per page for smoother loading
let startIndex = 0;
let isFetching = false; // Flag to prevent concurrent fetches

// Function to fetch data from the server
async function fetchData(start, end) {
    if (isFetching) return; // Prevent fetching if already fetching
    isFetching = true;
    loader.style.display = 'block'; // Show loading indicator
    container.classList.add('loading'); // Dim the content

    try {
        const response = await fetch("http://localhost:8080/cache/ips?start=" + start + "&end=" + end);
        const data = await response.json();
        return data;
    } catch (error) {
        console.error("Failed to fetch data:", error);
        return []; // Return an empty array on error
    } finally {
        isFetching = false;
        loader.style.display = 'none'; // Hide loading indicator
        container.classList.remove('loading'); // Restore content visibility
    }
}

// Function to append data to the list
function appendData(data) {
    data.forEach(item => {
        const li = document.createElement('li');
        li.textContent = item;
        dataList.appendChild(li);
    });
}

// Function to update the scrollbar thumb position and size
function updateScrollbar() {
    const scrollTop = container.scrollTop;
    const scrollHeight = container.scrollHeight - container.clientHeight;
    const scrollPercent = scrollTop / scrollHeight;

    const thumbHeight = Math.max(20, container.clientHeight * (container.clientHeight / container.scrollHeight));
    const thumbTop = scrollPercent * (container.clientHeight - thumbHeight);

    thumb.style.height = thumbHeight + "px";
    thumb.style.top = thumbTop + "px";
}

// Initial data fetch and render
fetchData(startIndex, startIndex + itemsPerPage)
    .then(data => {
        appendData(data);
        updateScrollbar();
        startIndex += data.length; // Update start index based on fetched data
    });

container.addEventListener('scroll', () => {
    const { scrollTop, scrollHeight, clientHeight } = container;

    // Check if the user has scrolled to the bottom
    if (scrollTop + clientHeight >= scrollHeight - 50) { // 50px threshold
        fetchData(startIndex, startIndex + itemsPerPage)
            .then(data => {
                if (data.length > 0) {
                    appendData(data);
                    updateScrollbar();
                    // currentColor = currentColor === '#3232a8' ? '#09b576' : '#3232a8';
	  		        // container.style.boxShadow = '-10px 10px 20px ' + currentColor;
                    startIndex += data.length; // Update start index based on fetched data
                }
            });
    }

    updateScrollbar();
});
function clearInput() {
    document.getElementById('search-input').value = '';
}

// Update scrollbar when the window is resized
window.addEventListener('resize', updateScrollbar);
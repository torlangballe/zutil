package zdesktop

import (
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zhttp"
	"image"
)

func GetAppNameOfBrowser(btype zdevice.BrowserType, fullName bool) string {
	return string(btype)
}

func GetIDScaleAndRectForWindowTitle(title, app string) (id string, scale int, rect zgeo.Rect, err error) {
	return title, 1, zgeo.RectFromWH(100, 100), nil
}

func SetWindowRectForTitle(title, app string, rect zgeo.Rect) error {
	return nil
}

func GetImageForWindowTitle(title, app string, crop zgeo.Rect) (image.Image, error) {
	/*

		    <canvas id="canvas" width="64" height="48"> </canvas>
		    <video id="capture-video" width="64" height="48"> </video>

				var captureStream = null;
				var imageCapture = null;


				async function startCapture() {
				    var options = {
				        video: {
				          cursor: "never",
				          displaySurface: "window"
				        },
				        audio: false,
				        preferCurrentTab: true
				      };

				    try {
				        // captureStream = await navigator.mediaDevices.getCurrentBrowsingContextMedia({audio: false, video: true});
				       captureStream = await navigator.mediaDevices.getDisplayMedia(options);
				    } catch(err) {
				      console.error("Error: " + err);
				    }
				}

		            console.log("capture");
		            var canvas = document.getElementById("canvas");
		            var context = canvas.getContext('2d');
		            if (imageCapture === null) {
		                let track = captureStream.getVideoTracks()[0]
		                imageCapture = new ImageCapture(track)
		            }
		            while(true) {
		                var wImg = await grabFrame();
		                if (wImg != null)
		                    context.drawImage(wImg, 0, 0, canvas.width, canvas.height);
		                await aSleep(100);
		            }

		            // const bitmap = await imageCapture.grabFrame()
		            // track.stop();
		            // context.drawImage(bitmap, 0, 0, canvas.width, canvas.height);

					async function aSleep(ms) {
		    return new Promise(R => setTimeout(R, ms));
		}

		let grabFrame = async function() {
		      var P = new Promise((R)=> {
		        imageCapture.grabFrame().then((vImg) => {
		          R(vImg);
		        }).catch((error) => {
		          R(null);
		        });
		      });
		      return P;
		    }

	*/
	return nil, nil
}

func CloseWindowForTitle(title, app string) error {
	return nil
}

func PrintWindowTitles() {

}

func CanGetWindowInfo() bool {
	return true
}

func CanControlComputer(prompt bool) bool {
	return true
}

func CanRecordScreen() bool {
	return true
}

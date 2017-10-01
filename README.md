# Atmospheric Scattering

Work in progress

# Work log

Chronological

1. First useful output

![white sphere](images/01_first_sphere.png)

2. White if the ray only hits the atmosphere, blue if it also hits in inner planet sphere

![two spheres](images/02_two_spheres.png)

3. Atmosphere greyscale intensity is proportional to linear distance ray travels through atmosphere without striking planet, black = little distance, white = furthest distance

![linear_distance_atmosphere](images/03_linear_distance_atmosphere.png)

04. No big change but the Earth and atmosphere are now using real values from Wikipedia. Camera moved back to fit Earth in the image.

![realistic_units](images/04_realistic_units.png)

05. Rotate the Earth around it's vertical axis

![05_transforms.png](images/05_transforms.png)

06. Add directional sunlight (white light)

![06_sunlight.png](images/06_sunlight.png)

07. Visualize optical length

![07_sunlight.png](images/07_optical_length.png)
